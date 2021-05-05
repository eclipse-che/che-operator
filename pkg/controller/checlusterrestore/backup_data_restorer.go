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

	cheCR.ObjectMeta.Namespace = rctx.namespace
	// Reset availability status
	cheCR.Status = orgv1.CheClusterStatus{}
	cheCR.Status.CheClusterRunning = ""

	// Adapt links in Che CR according to new cluster settings
	if rctx.isOpenShift {
		// TODO find a way to detect whether CheHost was set to custom value.
		// Since it is not possible to detect if CheHost contains custom value or just old cluster host,
		// reset it and let operator automatically set default value.
		cheCR.Spec.Server.CheHost = ""
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
		if strings.HasSuffix(dumpFile.Name(), ".pgdump") {
			dbDumpFilePath := path.Join(dumpsDir, dumpFile.Name())
			dumpBytes, err := ioutil.ReadFile(dbDumpFilePath)
			if err != nil {
				return false, err
			}
			dumpReader := bytes.NewReader(dumpBytes)

			execReason := fmt.Sprintf("restoring %s database", strings.TrimSuffix(dumpFile.Name(), ".pgdump"))
			restoreDumpRemoteCommand := getRestoreDatabasesScript(dumpFile.Name())
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

func getRestoreDatabasesScript(dbName string) string {
	return "DB_NAME=" + dbName + `
	  DUMP_FILE=/tmp/dbdump
		rm -f $DUMP_FILE
		cat > $DUMP_FILE
		psql -c "ALTER DATABASE ${DB_NAME} CONNECTION LIMIT 0;"
		psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${DB_NAME}';"
		psql -c "DROP DATABASE ${DB_NAME};"
		pg_restore -d postgres --create $DUMP_FILE
		rm -f $DUMP_FILE
	`
}
