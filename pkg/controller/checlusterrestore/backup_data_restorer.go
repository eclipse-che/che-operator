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
	"io/ioutil"
	"path"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func RestoreChe(rctx *RestoreContext, dataDir string) (bool, error) {
	// TODO deploy Che if there is no installation at all?

	// TODO should we delete CRDs ? Check for versions compatibility then.
	done, err := cleanPreviousInstallation(rctx)
	if err != nil || !done {
		return done, err
	}

	done, err = restoreDatabase(rctx, dataDir)
	if err != nil || !done {
		return done, err
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

func restoreDatabase(rctx *RestoreContext, dataDir string) (bool, error) {
	dumpsDir := path.Join(dataDir, "db")

	k8sClient := util.GetK8Client()
	postgresPodName, err := k8sClient.GetDeploymentPod(deploy.PostgresName, rctx.namespace)
	if err != nil {
		return false, err
	}

	items, _ := ioutil.ReadDir(dumpsDir)
	for _, item := range items {
		if !item.IsDir() && strings.HasSuffix(item.Name(), ".pgdump") {
			dbDumpFilePath := path.Join(dumpsDir, item.Name())
			dumpBytes, err := ioutil.ReadFile(dbDumpFilePath)
			if err != nil {
				return false, err
			}
			dump := bytes.NewReader(dumpBytes)

			restoreDumpRemoteCommand := getRestoreDatabasesScript(dbDumpFilePath)
			if output, err := k8sClient.DoExecIntoPod(rctx.namespace, postgresPodName, restoreDumpRemoteCommand, ""); err != nil {
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
