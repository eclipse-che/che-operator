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
package postgres

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func GetWaitForPostgresInitContainer(deployContext *chetypes.DeployContext) (*corev1.Container, error) {
	postgresDeployment := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(deployContext, constants.PostgresName, postgresDeployment)
	if err != nil {
		return nil, err
	}
	if !exists {
		postgresDeployment = nil
	}
	postgresReadinessCheckerImage, err := getPostgresImage(postgresDeployment, deployContext.CheCluster)
	if err != nil {
		return nil, err
	}
	imagePullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(postgresReadinessCheckerImage))

	return &corev1.Container{
		Name:            "wait-for-postgres",
		Image:           postgresReadinessCheckerImage,
		ImagePullPolicy: imagePullPolicy,
		Command: []string{
			"/bin/sh",
			"-c",
			getCheckPostgresReadinessScript(deployContext),
		},
	}, nil
}

func getCheckPostgresReadinessScript(deployContext *chetypes.DeployContext) string {
	chePostgresHostName := utils.GetValue(deployContext.CheCluster.Spec.Components.Database.PostgresHostName, constants.DefaultPostgresHostName)
	chePostgresPort := utils.GetValue(deployContext.CheCluster.Spec.Components.Database.PostgresPort, constants.DefaultPostgresPort)

	return fmt.Sprintf(
		"until pg_isready -h %s -p %s; do echo 'waiting for Postgres'; sleep 2; done;",
		chePostgresHostName,
		chePostgresPort)
}
