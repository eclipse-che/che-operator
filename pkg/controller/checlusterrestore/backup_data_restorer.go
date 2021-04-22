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
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func RestoreChe(rctx *RestoreContext, dataDir string) (bool, error) {
	// Make sure a Che instance is running and available
	if !rctx.state.oldCheAvailable {
		if rctx.cheCR == nil {
			// Deploy Che
			done, err := restoreCheCR(rctx, dataDir)
			if err != nil || !done {
				return done, err
			}

			return false, nil
		}

		if rctx.cheCR.Status.CheClusterRunning != "Available" {
			// Che is not ready yet
			return false, nil
		}

		rctx.state.oldCheAvailable = true
		rctx.UpdateRestoreStage()
	}

	// Stop existing Che reconciling loop and server itself
	if !rctx.state.oldCheSuspended {
		done, err := cleanPreviousInstallation(rctx)
		if err != nil || !done {
			return done, err
		}

		rctx.state.oldCheSuspended = true
		rctx.UpdateRestoreStage()
	}

	// Restore additional cluster objects from the backup
	if !rctx.state.cheResourcesRestored {
		done, err := restoreConfigMaps(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}

		rctx.state.cheResourcesRestored = true
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

	// Restore Che CR to start main controller reconcile loop
	if !rctx.state.cheCRRestored {
		done, err := restoreCheCR(rctx, dataDir)
		if err != nil || !done {
			return done, err
		}
		logrus.Info("Che is successfully restored. Wait until Che server is ready.")

		rctx.state.cheCRRestored = true
		rctx.UpdateRestoreStage()
	}

	// Wait until Che available again
	if !rctx.state.cheRestored {
		if rctx.cheCR.Status.CheClusterRunning != "Available" {
			return false, nil
		}

		rctx.state.cheRestored = true
		rctx.UpdateRestoreStage()
	}

	return true, nil
}

func cleanPreviousInstallation(rctx *RestoreContext) (bool, error) {
	// Delete Che CR to stop operator from dealing with current installation
	err := rctx.r.client.Delete(context.TODO(), rctx.cheCR)
	if err != nil && !errors.IsNotFound(err) {
		return false, err
	}

	done, err := deleteDeployments(rctx)
	if err != nil || !done {
		return done, err
	}

	done, err = deleteConfigMaps(rctx)
	if err != nil || !done {
		return done, err
	}

	return true, nil
}

func deleteDeployments(rctx *RestoreContext) (bool, error) {
	// Delete Che deployment to prevent new queries into database
	cheFlavor := deploy.DefaultCheFlavor(rctx.cheCR)

	cheNamespacedName := types.NamespacedName{Namespace: rctx.namespace, Name: cheFlavor}
	cheDeployment := &appsv1.Deployment{}
	err := rctx.r.client.Get(context.TODO(), cheNamespacedName, cheDeployment)
	if err == nil {
		if err := rctx.r.client.Delete(context.TODO(), cheDeployment); err != nil {
			return false, err
		}
	} else if !errors.IsNotFound(err) {
		return false, err
	}

	// The same for Keycloak
	keycloakNamespacedName := types.NamespacedName{Namespace: rctx.namespace, Name: deploy.IdentityProviderName}
	keycloakDeployment := &appsv1.Deployment{}
	err = rctx.r.client.Get(context.TODO(), keycloakNamespacedName, keycloakDeployment)
	if err == nil {
		if err := rctx.r.client.Delete(context.TODO(), keycloakDeployment); err != nil {
			return false, err
		}
	} else if !errors.IsNotFound(err) {
		return false, err
	}

	return true, nil
}

func deleteConfigMaps(rctx *RestoreContext) (bool, error) {
	// Delete all configmaps with custom CA certificates
	err := rctx.r.client.DeleteAllOf(context.TODO(), &corev1.ConfigMap{}, client.InNamespace(rctx.namespace),
		client.MatchingLabels{
			deploy.CheCACertsConfigMapLabelKey: deploy.CheCACertsConfigMapLabelValue,
			deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		})
	if err != nil {
		return false, err
	}

	return true, nil
}

func restoreConfigMaps(rctx *RestoreContext, dataDir string) (bool, error) {
	configMapsDir := path.Join(dataDir, "configmaps")
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

		configMapBytes, err := ioutil.ReadFile(cmFile.Name())
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

func restoreDatabase(rctx *RestoreContext, dataDir string) (bool, error) {
	dumpsDir := path.Join(dataDir, "db")

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

			restoreDumpRemoteCommand := getRestoreDatabasesScript(dbDumpFilePath)
			if output, err := k8sClient.DoExecIntoPodWithStdin(rctx.namespace, postgresPodName, restoreDumpRemoteCommand, dumpReader, ""); err != nil {
				logrus.Error(output)
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
		dropdb $DB_NAME
		pg_restore --create --dbname $DB_NAME $DUMP_FILE
		rm -f $DUMP_FILE
	`
}

func restoreCheCR(rctx *RestoreContext, dataDir string) (bool, error) {
	cheCRFilePath := path.Join(dataDir, "che-cr.yaml")
	if _, err := os.Stat(cheCRFilePath); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		// Cannot proceed without CR in backup data
		return true, err
	}

	cheCR := &orgv1.CheCluster{}
	cheCRBytes, err := ioutil.ReadFile(cheCRFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to read Che CR from '%s' file", cheCRFilePath)
	}
	if err := yaml.Unmarshal(cheCRBytes, cheCR); err != nil {
		return true, err
	}

	cheCR.ObjectMeta.Namespace = rctx.namespace
	// Correct ingress domain if requested
	if isOpenShift, _, _ := util.DetectOpenShift(); !isOpenShift {
		if rctx.restoreCR.Spec.CROverrides.IngressDomain != "" {
			cheCR.Spec.K8s.IngressDomain = rctx.restoreCR.Spec.CROverrides.IngressDomain
		}
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
