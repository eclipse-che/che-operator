//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package checlusterrestore

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/controllers/checlusterbackup"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func RestoreChe(rctx *RestoreContext, dataDir string) (bool, error) {
	// Request deletion of existing Che resources if any
	if !rctx.state.oldCheDeletionRequested {
		done, err := cleanPreviousInstallation(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.oldCheDeletionRequested = true
		if err := rctx.UpdateRestoreStatus(); err != nil {
			return false, err
		}
	}

	// Waiting for finalizers of existing Che if any
	if !rctx.state.oldCheCleaned {
		done, err := waitPreviousInstallationDeleted(rctx)
		if err != nil || !done {
			logrus.Info("Restore: Waiting for existing Che to be deleted")
			return done, err
		}

		rctx.state.oldCheCleaned = true
		if err := rctx.UpdateRestoreStatus(); err != nil {
			return false, err
		}
	}

	// Restore cluster objects from the backup
	if !rctx.state.cheResourcesRestored {
		done, err := restoreCheResources(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.cheResourcesRestored = true
		if err := rctx.UpdateRestoreStatus(); err != nil {
			return false, err
		}
	}

	// Restore Che CR to start main controller reconcile loop
	if !rctx.state.cheCRRestored {
		done, err := restoreCheCR(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.cheCRRestored = true
		if err := rctx.UpdateRestoreStatus(); err != nil {
			return false, err
		}
	}

	// Wait until Che deployed and ready
	if !rctx.state.cheAvailable {
		if rctx.cheCR == nil || rctx.cheCR.Status.CheClusterRunning != "Available" {
			logrus.Info("Restore: Waiting for Che to be ready")
			return false, nil
		}

		rctx.state.cheAvailable = true
		if err := rctx.UpdateRestoreStatus(); err != nil {
			return false, err
		}
	}

	// Restore database from backup dump
	if !rctx.state.cheDatabaseRestored {
		done, err := restoreDatabase(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.cheDatabaseRestored = true
		if err := rctx.UpdateRestoreStatus(); err != nil {
			return false, err
		}
	}

	return true, nil
}

func cleanPreviousInstallation(rctx *RestoreContext, dataDir string) (bool, error) {
	if rctx.cheCR == nil {
		// If there is no CR in the cluster, then use one from the backup.
		// This is needed to be able to clean some related resources if any.
		cheCR, done, err := readAndAdaptCheCRFromBackup(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}
		rctx.cheCR = cheCR
	}

	// Drop databases, if any, to handle case of different Postgres version (otherwise clean Che will fail to start due to old data in volume)
	dropDatabases(rctx)

	// Delete Che CR to stop operator from dealing with current installation
	actualCheCR, cheCRCount, err := util.FindCheClusterCRInNamespace(rctx.r.cachingClient, rctx.namespace)
	if cheCRCount == -1 {
		// error occurred while retreiving CheCluster CR
		return false, err
	} else if actualCheCR != nil {
		if actualCheCR.GetObjectMeta().GetDeletionTimestamp().IsZero() {
			logrus.Infof("Restore: Deleteing CheCluster custom resource in '%s' namespace", rctx.namespace)
			err := rctx.r.cachingClient.Delete(context.TODO(), actualCheCR)
			if err == nil {
				// Che CR is marked for deletion, but actually still exists.
				// Wait for finalizers and actual resource deletion (not found expected).
				logrus.Info("Restore: Waiting for old Che CR finalizers to be completed")
				return false, nil
			} else if !errors.IsNotFound(err) {
				return false, err
			}
		} else {
			return false, nil
		}
	}

	// Define label selector for resources to clean up
	cheFlavor := deploy.DefaultCheFlavor(rctx.cheCR)

	cheNameRequirement, _ := labels.NewRequirement(deploy.KubernetesNameLabelKey, selection.Equals, []string{cheFlavor})
	cheInstanceRequirement, _ := labels.NewRequirement(deploy.KubernetesInstanceLabelKey, selection.Equals, []string{cheFlavor})
	skipBackupObjectsRequirement, _ := labels.NewRequirement(deploy.KubernetesPartOfLabelKey, selection.NotEquals, []string{checlusterbackup.BackupCheEclipseOrg})

	cheResourcesLabelSelector := labels.NewSelector().Add(*cheInstanceRequirement).Add(*cheNameRequirement).Add(*skipBackupObjectsRequirement)
	cheResourcesListOptions := &client.ListOptions{
		LabelSelector: cheResourcesLabelSelector,
		Namespace:     rctx.namespace,
	}
	cheResourcesMatchingLabelsSelector := client.MatchingLabelsSelector{Selector: cheResourcesLabelSelector}

	// Delete all Che related deployments, but keep operator (excluded by name) and internal backup server (excluded by label)
	deploymentsList := &appsv1.DeploymentList{}
	if err := rctx.r.nonCachingClient.List(context.TODO(), deploymentsList, cheResourcesListOptions); err != nil {
		return false, err
	}
	for _, deployment := range deploymentsList.Items {
		if strings.Contains(deployment.GetName(), cheFlavor+"-operator") {
			continue
		}
		if err := rctx.r.nonCachingClient.Delete(context.TODO(), &deployment); err != nil && !errors.IsNotFound(err) {
			return false, err
		}
	}

	// Delete all Che related secrets, but keep backup server ones (excluded by label)
	if err := rctx.r.nonCachingClient.DeleteAllOf(context.TODO(), &corev1.Secret{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	// Delete all configmaps with custom CA certificates
	if err := rctx.r.nonCachingClient.DeleteAllOf(context.TODO(), &corev1.ConfigMap{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	// Delete all Che related config maps
	if err := rctx.r.nonCachingClient.DeleteAllOf(context.TODO(), &corev1.ConfigMap{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	// Delete all Che related ingresses / routes
	if rctx.isOpenShift {
		err = rctx.r.nonCachingClient.DeleteAllOf(context.TODO(), &routev1.Route{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector)
	} else {
		err = rctx.r.nonCachingClient.DeleteAllOf(context.TODO(), &networking.Ingress{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector)
	}
	if err != nil {
		return false, err
	}

	// Delete all Che related persistent volumes
	if err := rctx.r.nonCachingClient.DeleteAllOf(context.TODO(), &corev1.PersistentVolumeClaim{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	return true, nil
}

func waitPreviousInstallationDeleted(rctx *RestoreContext) (bool, error) {
	_, cheCRCount, err := util.FindCheClusterCRInNamespace(rctx.r.cachingClient, rctx.namespace)
	if cheCRCount == -1 {
		// An error occurred while retreiving CheCluster CR
		return false, err
	} else if cheCRCount > 0 {
		// Wait more for finalizers
		return false, nil
	}
	// Che CR is deleted
	return true, nil
}

func restoreCheResources(rctx *RestoreContext, dataDir string) (bool, error) {
	partsToRestore := []func(*RestoreContext, string) (bool, error){
		restoreConfigMaps,
		restoreSecrets,
	}

	for _, restorePart := range partsToRestore {
		done, err := restorePart(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}
	}

	return true, nil
}

func restoreConfigMaps(rctx *RestoreContext, dataDir string) (bool, error) {
	configMapsDir := path.Join(dataDir, checlusterbackup.BackupConfigMapsDir)
	if _, err := os.Stat(configMapsDir); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		// Consider there is nothing to restore here
		return true, nil
	}

	configMaps, _ := ioutil.ReadDir(configMapsDir)
	for _, cmFile := range configMaps {
		configMap := &corev1.ConfigMap{}

		configMapBytes, err := ioutil.ReadFile(path.Join(configMapsDir, cmFile.Name()))
		if err != nil {
			return false, fmt.Errorf("failed to read Config Map from '%s' file", cmFile.Name())
		}
		if err := yaml.Unmarshal(configMapBytes, configMap); err != nil {
			return true, err
		}

		configMap.ObjectMeta.Namespace = rctx.namespace

		if err := rctx.r.nonCachingClient.Create(context.TODO(), configMap); err != nil {
			if !errors.IsAlreadyExists(err) {
				return false, err
			}

			if err := rctx.r.nonCachingClient.Delete(context.TODO(), configMap); err != nil {
				return false, err
			}
			if err := rctx.r.nonCachingClient.Create(context.TODO(), configMap); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func restoreSecrets(rctx *RestoreContext, dataDir string) (bool, error) {
	secretsDir := path.Join(dataDir, checlusterbackup.BackupSecretsDir)
	if _, err := os.Stat(secretsDir); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		// Consider there is nothing to restore here
		return true, nil
	}

	secrets, _ := ioutil.ReadDir(secretsDir)
	for _, secretFile := range secrets {
		secret := &corev1.Secret{}

		secretBytes, err := ioutil.ReadFile(path.Join(secretsDir, secretFile.Name()))
		if err != nil {
			return false, fmt.Errorf("failed to read Secret from '%s' file", secretFile.Name())
		}
		if err := yaml.Unmarshal(secretBytes, secret); err != nil {
			return true, err
		}

		secret.ObjectMeta.Namespace = rctx.namespace
		secret.ObjectMeta.OwnerReferences = nil

		if err := rctx.r.nonCachingClient.Create(context.TODO(), secret); err != nil {
			if !errors.IsAlreadyExists(err) {
				return false, err
			}

			if err := rctx.r.nonCachingClient.Delete(context.TODO(), secret); err != nil {
				return false, err
			}
			if err := rctx.r.nonCachingClient.Create(context.TODO(), secret); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func restoreCheCR(rctx *RestoreContext, dataDir string) (bool, error) {
	cheCR, done, err := readAndAdaptCheCRFromBackup(rctx, dataDir)
	if err != nil || !done {
		return done, err
	}

	if err := rctx.r.cachingClient.Create(context.TODO(), cheCR); err != nil {
		if errors.IsAlreadyExists(err) {
			// We should take into account that every step can be executed several times due to async behavior.
			// 1. We ensured that CheCluster is removed before restoring.
			// 2. If it is already created then it is safe to continue (was created here on a previous reconcile loop)
			return true, nil
		}
		return false, err
	}

	logrus.Info("Restore: CheCluster custom resource created")

	rctx.cheCR = cheCR
	return true, nil
}

func readAndAdaptCheCRFromBackup(rctx *RestoreContext, dataDir string) (*chev1.CheCluster, bool, error) {
	cheCR, done, err := readCheCR(rctx, dataDir)
	if err != nil || !done {
		return nil, done, err
	}

	cheCR.ObjectMeta.Namespace = rctx.namespace
	// Reset availability status
	cheCR.Status = chev1.CheClusterStatus{}
	cheCR.Status.CheClusterRunning = ""

	// Adapt links in Che CR according to new cluster settings
	if rctx.isOpenShift {
		backupMetadata, done, err := readBackupMetadata(rctx, dataDir)
		if err != nil || !done {
			return nil, done, err
		}
		if strings.Contains(cheCR.Spec.Server.CheHost, backupMetadata.AppsDomain) {
			// Reset to let operator put the correct value as previos one was autogenerated too.
			cheCR.Spec.Server.CheHost = ""
		}
	} else {
		// Check if Che host has custom value
		if strings.Contains(cheCR.Spec.Server.CheHost, cheCR.Spec.K8s.IngressDomain) {
			// CheHost was generated by operator.
			// Reset it to let the operator put the correct value according to the new settings (domain and namespace).
			cheCR.Spec.Server.CheHost = ""
		}
	}

	return cheCR, true, nil
}

func readCheCR(rctx *RestoreContext, dataDir string) (*chev1.CheCluster, bool, error) {
	cheCRFilePath := path.Join(dataDir, checlusterbackup.BackupCheCRFileName)
	if _, err := os.Stat(cheCRFilePath); err != nil {
		if !os.IsNotExist(err) {
			return nil, false, err
		}
		// Cannot proceed without CR in backup data
		return nil, true, err
	}

	cheCR := &chev1.CheCluster{}
	cheCRBytes, err := ioutil.ReadFile(cheCRFilePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read Che CR from '%s' file", cheCRFilePath)
	}
	if err := yaml.Unmarshal(cheCRBytes, cheCR); err != nil {
		return nil, true, err
	}

	return cheCR, true, nil
}

func readBackupMetadata(rctx *RestoreContext, dataDir string) (*checlusterbackup.BackupMetadata, bool, error) {
	backupMetadataFilePath := path.Join(dataDir, checlusterbackup.BackupMetadataFileName)
	if _, err := os.Stat(backupMetadataFilePath); err != nil {
		if !os.IsNotExist(err) {
			return nil, false, err
		}
		return nil, true, err
	}

	backupMetadata := &checlusterbackup.BackupMetadata{}
	backupMetadataBytes, err := ioutil.ReadFile(backupMetadataFilePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read backup metadata from '%s' file", backupMetadataFilePath)
	}
	if err := yaml.Unmarshal(backupMetadataBytes, backupMetadata); err != nil {
		return nil, true, err
	}

	return backupMetadata, true, nil
}

// dropDatabases deletes Che related databases from Postgres, if any
func dropDatabases(rctx *RestoreContext) {
	if rctx.cheCR.Spec.Database.ExternalDb {
		// Skip this step as there is an external server to connect to
		return
	}

	databasesToDrop := []string{
		rctx.cheCR.Spec.Database.ChePostgresDb,
	}

	k8sClient := util.GetK8Client()
	postgresPodName, err := k8sClient.GetDeploymentPod(deploy.PostgresName, rctx.namespace)
	if err != nil {
		// Postgres pod not found, probably it doesn't exist
		return
	}

	for _, dbName := range databasesToDrop {
		execReason := fmt.Sprintf("dropping %s database", dbName)
		dropDatabaseCommand := fmt.Sprintf("psql -c \"DROP DATABASE IF EXISTS %s;\"", dbName)
		if output, err := k8sClient.DoExecIntoPod(rctx.namespace, postgresPodName, dropDatabaseCommand, execReason); err != nil {
			if output != "" {
				logrus.Error(output)
			}
		}
	}
}

func restoreDatabase(rctx *RestoreContext, dataDir string) (bool, error) {
	if rctx.cheCR.Spec.Database.ExternalDb {
		// Skip database restore as there is an external server to connect to
		return true, nil
	}

	dumpsDir := path.Join(dataDir, checlusterbackup.BackupDatabasesDir)

	k8sClient := util.GetK8Client()
	postgresPodName, err := k8sClient.GetDeploymentPod(deploy.PostgresName, rctx.namespace)
	if err != nil {
		return false, err
	}

	dumps, _ := ioutil.ReadDir(dumpsDir)
	for _, dumpFile := range dumps {
		if strings.HasSuffix(dumpFile.Name(), ".sql") {
			dbDumpFilePath := path.Join(dumpsDir, dumpFile.Name())
			dumpBytes, err := ioutil.ReadFile(dbDumpFilePath)
			if err != nil {
				return false, err
			}
			dumpReader := bytes.NewReader(dumpBytes)

			// Database name is the dump file name without extension
			dbName := strings.TrimSuffix(dumpFile.Name(), ".sql")
			dbOwner, err := getDatabaseOwner(rctx, dbName)
			if err != nil {
				return false, err
			}
			execReason := fmt.Sprintf("restoring %s database", dbName)
			restoreDumpRemoteCommand := getRestoreDatabaseScript(dbName, dbOwner)
			if output, err := k8sClient.DoExecIntoPodWithStdin(rctx.namespace, postgresPodName, restoreDumpRemoteCommand, dumpReader, execReason); err != nil {
				if output != "" {
					logrus.Error(output)
				}
				return false, err
			}
		}
	}

	return true, nil
}

func getDatabaseOwner(rctx *RestoreContext, dbName string) (string, error) {
	switch dbName {
	case rctx.cheCR.Spec.Database.ChePostgresDb:
		if rctx.cheCR.Spec.Database.ChePostgresUser != "" && rctx.cheCR.Spec.Database.ChePostgresPassword != "" {
			return rctx.cheCR.Spec.Database.ChePostgresUser, nil
		}

		secret := &corev1.Secret{}
		chePostgresCredentialSecretNsN := types.NamespacedName{
			Name:      rctx.cheCR.Spec.Database.ChePostgresSecret,
			Namespace: rctx.namespace,
		}
		if err := rctx.r.nonCachingClient.Get(context.TODO(), chePostgresCredentialSecretNsN, secret); err != nil {
			return "", err
		}
		return string(secret.Data["user"]), nil
	default:
		return "postgres", nil
	}
}

func getRestoreDatabaseScript(dbName string, dbOwner string) string {
	return "DB_NAME='" + dbName + "' \n DB_OWNER='" + dbOwner + "' \n" + `
	  DUMP_FILE=/tmp/dbdump.sql
		cat > $DUMP_FILE
		psql -c "ALTER DATABASE ${DB_NAME} CONNECTION LIMIT 0;"
		psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}';"
		psql -c "DROP DATABASE ${DB_NAME};"
		createdb "$DB_NAME" --owner="$DB_OWNER"
		psql "$DB_NAME" < "$DUMP_FILE"
		rm -f "$DUMP_FILE"
	`
}

func getPatchDatabaseScript(rctx *RestoreContext, dbName string, dataDir string) (string, error) {
	backupMetadata, done, err := readBackupMetadata(rctx, dataDir)
	if err != nil || !done {
		return "", err
	}

	oldNamespace := backupMetadata.Namespace
	newNamespace := rctx.namespace
	appsDomain := ""
	if rctx.isOpenShift {
		appsDomain, err = util.GetRouterCanonicalHostname(rctx.r.nonCachingClient, rctx.namespace)
		if err != nil {
			return "", err
		}
	} else {
		appsDomain = rctx.cheCR.Spec.K8s.IngressDomain
	}

	oldAppsSubStr := oldNamespace + "." + appsDomain
	newAppsSubStr := newNamespace + "." + appsDomain
	// namespace1.192.168.99.253.nip.io -> namespace2.192.168.99.253.nip.io
	// in, for example, https://devfile-registry-namespace1.192.168.99.253.nip.io/resources/cpp-cpp-hello-world-master.zip
	shouldPatchUrls := oldAppsSubStr != newAppsSubStr

	switch dbName {
	case rctx.cheCR.Spec.Database.ChePostgresDb:
		script := getCleanRunningWorkspacesScript(dbName)
		if shouldPatchUrls {
			script += "\n" + getReplaceInColumnScript(dbName, "devfile_project", "location", oldAppsSubStr, newAppsSubStr)
		}
		return script, nil
	}
	return "", nil
}

func getReplaceInColumnScript(dbName string, table string, column string, oldSubStr string, newSubStr string) string {
	query := fmt.Sprintf("UPDATE %s SET %s = REPLACE (%s, '%s', '%s');", table, column, column, oldSubStr, newSubStr)
	return fmt.Sprintf("psql %s -c \"%s\"", dbName, query)
}

// getCleanRunningWorkspacesScript returns script that clears workspace runtime related tables,
// so Che server doesn't think that some workspaces are running any more.
func getCleanRunningWorkspacesScript(dbName string) string {
	tables := [...]string{
		"che_k8s_runtime",
		"che_k8s_machine",
		"che_k8s_machine_attributes",
		"che_k8s_server",
		"che_k8s_server_attributes",
		"k8s_runtime_command",
		"k8s_runtime_command_attributes",
	}
	query := "TRUNCATE " + strings.Join(tables[:], ", ")
	return fmt.Sprintf("psql %s -c \"%s\"", dbName, query)
}
