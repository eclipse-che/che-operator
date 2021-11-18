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

package che

import (
	"context"
	"fmt"
	"strings"

	semver "github.com/blang/semver/v4"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
)

const (
	DefaultBackupServerConfigLabelKey = "che.eclipse.org/backup-before-update"
)

func isCheGoingToBeUpdated(cheCR *chev1.CheCluster) bool {
	deployedCheVersion := cheCR.Status.CheVersion
	newCheVersion := deploy.DefaultCheVersion()
	if deployedCheVersion == "" || deployedCheVersion == "next" || deployedCheVersion == newCheVersion {
		return false
	}

	deployedSemver, err := semver.Make(deployedCheVersion)
	if err != nil {
		logrus.Warn(getSemverParseErrorMessage(deployedCheVersion, err))
		return false
	}
	currentSemver, err := semver.Make(newCheVersion)
	if err != nil {
		logrus.Warn(getSemverParseErrorMessage(newCheVersion, err))
		return false
	}
	return currentSemver.GT(deployedSemver)
}

func getSemverParseErrorMessage(version string, err error) string {
	return fmt.Sprintf("It is not possible to parse a version '%s'. Cause: %v", version, err)
}

// getBackupCRForUpdate returns backup CR that corresponds to the backup made before updating to a new Che version.
func getBackupCRForUpdate(deployContext *deploy.DeployContext) (*chev1.CheClusterBackup, error) {
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

func requestBackup(deployContext *deploy.DeployContext) error {
	backupCR, err := getBackupCRSpec(deployContext)
	if err != nil {
		return err
	}
	return deployContext.ClusterAPI.Client.Create(context.TODO(), backupCR)
}

func getBackupCRSpec(deployContext *deploy.DeployContext) (*chev1.CheClusterBackup, error) {
	backupServerConfigName, err := getBackupServerConfigurationNameForBackupBeforeUpdate(deployContext)
	if err != nil {
		return nil, err
	}
	cheClusterBackupSpec := chev1.CheClusterBackupSpec{}
	if backupServerConfigName == "" {
		cheClusterBackupSpec.UseInternalBackupServer = true
	} else {
		cheClusterBackupSpec.UseInternalBackupServer = false
		cheClusterBackupSpec.BackupServerConfigRef = backupServerConfigName
	}

	return &chev1.CheClusterBackup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheClusterBackup",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getBackupCRNameForVersion(deploy.DefaultCheVersion()),
			Namespace: deployContext.CheCluster.GetNamespace(),
		},
		Spec: cheClusterBackupSpec,
	}, nil
}

// getBackupServerConfigurationNameForBackupBeforeUpdate searches for backup server configuration.
// If there is only one, then it is used.
// If there are two or more, then one with 'che.eclipse.org/default-backup-server-configuration' annotation is used.
// If there is none, then empty string is returned.
func getBackupServerConfigurationNameForBackupBeforeUpdate(deployContext *deploy.DeployContext) (string, error) {
	backupServerConfigsList := &chev1.CheBackupServerConfigurationList{}
	listOptions := &client.ListOptions{
		Namespace: deployContext.CheCluster.GetNamespace(),
	}
	if err := deployContext.ClusterAPI.Client.List(context.TODO(), backupServerConfigsList, listOptions); err != nil {
		return "", err
	}
	if len(backupServerConfigsList.Items) == 1 {
		return backupServerConfigsList.Items[0].GetName(), nil
	}
	for _, backupServerConfig := range backupServerConfigsList.Items {
		if backupServerConfig.ObjectMeta.Annotations != nil && backupServerConfig.ObjectMeta.Annotations[DefaultBackupServerConfigLabelKey] == "true" {
			return backupServerConfig.GetName(), nil
		}
	}
	return "", nil
}
