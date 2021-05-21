//
// Copyright (c) 2021 Red Hat, Inc.
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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/controller/checlusterbackup"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func RestoreChe(rctx *RestoreContext, dataDir string) (bool, error) {
	// Delete existing Che resources if any
	if !rctx.state.oldCheCleaned {
		done, err := cleanPreviousInstallation(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.oldCheCleaned = true
		rctx.UpdateRestoreStage()
	}

	// Restore cluster objects from the backup
	if !rctx.state.cheResourcesRestored {
		done, err := restoreCheResources(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.cheResourcesRestored = true
		rctx.UpdateRestoreStage()
	}

	// Restore Che CR to start main controller reconcile loop
	if !rctx.state.cheCRRestored {
		done, err := restoreCheCR(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.cheCRRestored = true
		rctx.UpdateRestoreStage()
	}

	// Wait until Che deployed and ready
	if !rctx.state.cheAvailable {
		if rctx.cheCR.Status.CheClusterRunning != "Available" {
			logrus.Info("Restore: Waiting for Che to be ready")
			return false, nil
		}

		rctx.state.cheAvailable = true
		rctx.UpdateRestoreStage()
	}

	// Restore database from backup dump
	if !rctx.state.cheDatabaseRestored {
		done, err := restoreDatabase(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		// After Keycloak's database restoring, it is required to restart Keycloak to invalidate its cache.
		done, err = deleteKeycloakPod(rctx)
		if err != nil || !done {
			return done, err
		}

		rctx.state.cheDatabaseRestored = true
		rctx.UpdateRestoreStage()
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

	// Delete Che CR to stop operator from dealing with current installation
	err := rctx.r.client.Delete(context.TODO(), rctx.cheCR)
	if err == nil {
		// Che CR is marked for deletion, but actually still exists.
		// Wait for finalizers and actual resource deletion (not found expected).
		logrus.Info("Restore: Waiting for old Che CR finalizers to be completed")
		return false, nil
	} else if !errors.IsNotFound(err) {
		return false, err
	}

	// Define label selector for resources to clean up
	cheFlavor := deploy.DefaultCheFlavor(rctx.cheCR)

	cheNameRequirement, _ := labels.NewRequirement(deploy.KubernetesNameLabelKey, selection.Equals, []string{cheFlavor})
	cheInstanceRequirement, _ := labels.NewRequirement(deploy.KubernetesInstanceLabelKey, selection.Equals, []string{cheFlavor})
	skipBackupObjectsRequirement, _ := labels.NewRequirement(deploy.KubernetesPartOfLabelKey, selection.NotEquals, []string{checlusterbackup.BackupCheEclipseOrg})

	cheResourcesLabelSelector := labels.NewSelector().Add(*cheInstanceRequirement).Add(*cheNameRequirement).Add(*skipBackupObjectsRequirement)
	cheResourcesListOptions := &client.ListOptions{LabelSelector: cheResourcesLabelSelector}
	cheResourcesMatchingLabelsSelector := client.MatchingLabelsSelector{Selector: cheResourcesLabelSelector}

	// Delete all Che related deployments, but keep operator (excluded by name) and internal backup server (excluded by label)
	deploymentsList := &appsv1.DeploymentList{}
	if err := rctx.r.client.List(context.TODO(), deploymentsList, cheResourcesListOptions); err != nil {
		return false, err
	}
	for _, deployment := range deploymentsList.Items {
		if strings.Contains(deployment.GetName(), cheFlavor+"-operator") {
			continue
		}
		if err := rctx.r.client.Delete(context.TODO(), &deployment); err != nil && !errors.IsNotFound(err) {
			return false, err
		}
	}

	// Delete all Che related secrets, but keep backup server ones (excluded by label)
	if err := rctx.r.client.DeleteAllOf(context.TODO(), &corev1.Secret{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	// Delete all configmaps with custom CA certificates
	if err := rctx.r.client.DeleteAllOf(context.TODO(), &corev1.ConfigMap{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	// Delete all Che related config maps
	if err := rctx.r.client.DeleteAllOf(context.TODO(), &corev1.ConfigMap{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	// Delete all Che related ingresses / routes
	if rctx.isOpenShift {
		err = rctx.r.client.DeleteAllOf(context.TODO(), &routev1.Route{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector)
	} else {
		err = rctx.r.client.DeleteAllOf(context.TODO(), &extv1beta1.Ingress{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector)
	}
	if err != nil {
		return false, err
	}

	// Delete all Che related persistent volumes
	if err := rctx.r.client.DeleteAllOf(context.TODO(), &corev1.PersistentVolumeClaim{}, client.InNamespace(rctx.namespace), cheResourcesMatchingLabelsSelector); err != nil {
		return false, err
	}

	return true, nil
}

func deleteKeycloakPod(rctx *RestoreContext) (bool, error) {
	k8sClient := util.GetK8Client()
	keycloakPodName, err := k8sClient.GetDeploymentPod(deploy.IdentityProviderName, rctx.namespace)
	if err != nil {
		return false, err
	}
	keycloakPodNsN := types.NamespacedName{Name: keycloakPodName, Namespace: rctx.namespace}
	keycloakPod := &corev1.Pod{}
	if err := rctx.r.client.Get(context.TODO(), keycloakPodNsN, keycloakPod); err != nil {
		return false, err
	}
	if err := rctx.r.client.Delete(context.TODO(), keycloakPod); err != nil {
		return false, err
	}
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

		if err := rctx.r.client.Create(context.TODO(), configMap); err != nil {
			if !errors.IsAlreadyExists(err) {
				return false, err
			}

			if err := rctx.r.client.Delete(context.TODO(), configMap); err != nil {
				return false, err
			}
			if err := rctx.r.client.Create(context.TODO(), configMap); err != nil {
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

		if err := rctx.r.client.Create(context.TODO(), secret); err != nil {
			if !errors.IsAlreadyExists(err) {
				return false, err
			}

			if err := rctx.r.client.Delete(context.TODO(), secret); err != nil {
				return false, err
			}
			if err := rctx.r.client.Create(context.TODO(), secret); err != nil {
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

	if err := rctx.r.client.Create(context.TODO(), cheCR); err != nil {
		if errors.IsAlreadyExists(err) {
			return false, rctx.r.client.Delete(context.TODO(), cheCR)
		}
		return false, err
	}

	rctx.cheCR = cheCR
	return true, nil
}

func readAndAdaptCheCRFromBackup(rctx *RestoreContext, dataDir string) (*orgv1.CheCluster, bool, error) {
	cheCR, done, err := readCheCR(rctx, dataDir)
	if err != nil || !done {
		return nil, done, err
	}

	cheCR.ObjectMeta.Namespace = rctx.namespace
	// Reset availability status
	cheCR.Status = orgv1.CheClusterStatus{}
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
		// Correct ingress domain if requested
		oldIngressDomain := cheCR.Spec.K8s.IngressDomain
		if rctx.restoreCR.Spec.CROverrides.IngressDomain != "" {
			cheCR.Spec.K8s.IngressDomain = rctx.restoreCR.Spec.CROverrides.IngressDomain
		}
		// Check if Che host has custom value
		if strings.Contains(cheCR.Spec.Server.CheHost, oldIngressDomain) || strings.Contains(cheCR.Spec.Server.CheHost, cheCR.Spec.K8s.IngressDomain) {
			// CheHost was generated by operator.
			// Reset it to let the operator put the correct value according to the new settings (domain and namespace).
			cheCR.Spec.Server.CheHost = ""
		}
	}
	if !cheCR.Spec.Auth.ExternalIdentityProvider {
		// Let operator set the URL automatically
		cheCR.Spec.Auth.IdentityProviderURL = ""
	}

	return cheCR, true, nil
}

func readCheCR(rctx *RestoreContext, dataDir string) (*orgv1.CheCluster, bool, error) {
	cheCRFilePath := path.Join(dataDir, checlusterbackup.BackupCheCRFileName)
	if _, err := os.Stat(cheCRFilePath); err != nil {
		if !os.IsNotExist(err) {
			return nil, false, err
		}
		// Cannot proceed without CR in backup data
		return nil, true, err
	}

	cheCR := &orgv1.CheCluster{}
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

			if rctx.cheCR.Spec.Server.ServerExposureStrategy == "multi-host" {
				// Some databases contain values bind to cluster and/or namespace
				// These values should be adjusted according to new environmant.
				pathcDatabaseScript, err := getPatchDatabaseScript(rctx, dbName, dataDir)
				if err != nil {
					return false, err
				}
				if pathcDatabaseScript != "" {
					execReason := fmt.Sprintf("patching %s database", dbName)
					if output, err := k8sClient.DoExecIntoPod(rctx.namespace, postgresPodName, pathcDatabaseScript, execReason); err != nil {
						if output != "" {
							logrus.Error(output)
						}
						return false, err
					}
				}
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
		if err := rctx.r.client.Get(context.TODO(), chePostgresCredentialSecretNsN, secret); err != nil {
			return "", err
		}
		return string(secret.Data["user"]), nil
	case "keycloak":
		return "keycloak", nil
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
	// namespace1.192.168.99.253.nip.io -> namespace2.192.168.99.254.nip.io
	// in, for example, https://devfile-registry-namespace1.192.168.99.253.nip.io/resources/cpp-cpp-hello-world-master.zip
	oldNamespace, oldAppsDomain, oldAPIDomain, newNamespace, newAppsDomain, newAPIDomain, err := getNamespaceDomainReplace(rctx, dataDir)
	if err != nil {
		return "", err
	}
	oldAppsSubStr := oldNamespace + "." + oldAppsDomain
	newAppsSubStr := newNamespace + "." + newAppsDomain
	if oldAppsSubStr == newAppsSubStr {
		// No need to do anything, restoring into the same cluster and the same namespace
		return "", nil
	}

	switch dbName {
	case rctx.cheCR.Spec.Database.ChePostgresDb:
		return getReplaceInColumnScript(dbName, "devfile_project", "location", oldAppsSubStr, newAppsSubStr), nil
	case "keycloak":
		script := getReplaceInColumnScript(dbName, "redirect_uris", "value", oldAppsSubStr, newAppsSubStr) + "\n" +
			getReplaceInColumnScript(dbName, "web_origins", "value", oldAppsSubStr, newAppsSubStr) + "\n" +
			getReplaceInColumnScript(dbName, "identity_provider_config", "value", oldAPIDomain, newAPIDomain)
		return script, nil
	}
	return "", nil
}

func getReplaceInColumnScript(dbName string, table string, column string, oldSubStr string, newSubStr string) string {
	query := fmt.Sprintf("UPDATE %s SET %s = REPLACE (%s, '%s', '%s');", table, column, column, oldSubStr, newSubStr)
	return fmt.Sprintf("psql %s -c \"%s\"", dbName, query)
}

// getNamespaceDomainReplace returns oldNamespace oldAppsDomain oldAPIDomain newNamespace newAppsDomain newAPIDomain
func getNamespaceDomainReplace(rctx *RestoreContext, dataDir string) (string, string, string, string, string, string, error) {
	backupMetadata, done, err := readBackupMetadata(rctx, dataDir)
	if err != nil || !done {
		return "", "", "", "", "", "", err
	}

	oldNamespace := backupMetadata.Namespace
	oldAppsDomain := backupMetadata.AppsDomain
	oldAPIDomain := backupMetadata.APIDomain

	newNamespace := rctx.namespace
	newAppsDomain := ""
	newAPIDomain := ""
	if rctx.isOpenShift {
		// Openshift
		newAppsDomain, err = util.GetRouterCanonicalHostname(rctx.r.client, rctx.namespace)
		if err != nil {
			return "", "", "", "", "", "", err
		}

		_, internalAPIUrl, err := util.GetOpenShiftAPIUrls()
		// https://api.subdomain.openshift.com:6443 -> api.subdomain.openshift.com
		newAPIDomain = strings.TrimPrefix(strings.Split(internalAPIUrl, ":")[1], "//")
		if err != nil {
			return "", "", "", "", "", "", err
		}
	} else {
		// Kubernetes
		if rctx.restoreCR.Spec.CROverrides.IngressDomain != "" {
			newAppsDomain = rctx.restoreCR.Spec.CROverrides.IngressDomain
		} else {
			newAppsDomain = oldAppsDomain
			if strings.HasPrefix(backupMetadata.Infrastructure, "Openshift") {
				// Restoring on Kubernetes while backup was done on Openshift
				// CROverrides.IngressDomain must be set
				logrus.Error("Restoring on Kubernetes when backup was done on Openshift requires IngressDomain to be set")
			}
		}

		newAPIDomain = newAppsDomain
	}

	return oldNamespace, oldAppsDomain, oldAPIDomain, newNamespace, newAppsDomain, newAPIDomain, nil
}
