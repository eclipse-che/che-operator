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
package che

import (
	"context"
	"strings"

	semver "github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
)

func isPreviousVersionDeployed(cheCR *chev1.CheCluster) bool {
	deployedVersion := cheCR.Status.CheVersion
	currentVersion := deploy.DefaultCheVersion()
	if deployedVersion == "" || deployedVersion == "next" || deployedVersion == currentVersion {
		return false
	}

	deployedSemver, err := semver.Make(deployedVersion)
	if err != nil {
		logrus.Error(err)
		return false
	}
	currentSemver, err := semver.Make(currentVersion)
	if err != nil {
		logrus.Error(err)
		return false
	}
	return currentSemver.GT(deployedSemver)
}

func getBackupCR(deployContext *deploy.DeployContext) (*chev1.CheClusterBackup, error) {
	backupCR := &chev1.CheClusterBackup{}
	backupCRName := getBackupCRNameForVersion(deploy.DefaultCheVersion())
	backupCRNamespacedName := types.NamespacedName{Namespace: deployContext.CheCluster.GetNamespace(), Name: backupCRName}
	if err := deployContext.ClusterAPI.Client.Get(context.TODO(), backupCRNamespacedName, backupCR); err != nil {
		return nil, err
	}
	return backupCR, nil
}

func getBackupCRNameForVersion(version string) string {
	return "backup-before-update-to-" + strings.Replace(version, ".", "-", -1)
}

func requestNewBackup(deployContext *deploy.DeployContext) error {
	backupCR := getBackupCRSpec(deployContext)
	err := deployContext.ClusterAPI.Client.Create(context.TODO(), backupCR)
	return err
}

func getBackupCRSpec(deployContext *deploy.DeployContext) *chev1.CheClusterBackup {
	labels := deploy.GetLabels(deployContext.CheCluster, "backup-on-update")

	return &chev1.CheClusterBackup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheClusterBackup",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getBackupCRNameForVersion(deploy.DefaultCheVersion()),
			Namespace: deployContext.CheCluster.GetNamespace(),
			Labels:    labels,
		},
		Spec: chev1.CheClusterBackupSpec{
			UseInternalBackupServer: true,
		},
	}
}
