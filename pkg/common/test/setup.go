//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package test

import (
	"os"
	"path/filepath"

	dwInfrastructure "github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	cheInfrastructure "github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test/fakeclusterapi"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func StartOn(infra cheInfrastructure.Type) {
	InitializeDefaults(infra)
	setupClusterApi()
}

func InitializeDefaults(infra cheInfrastructure.Type) {
	cheInfrastructure.InitializeForTesting(infra)
	dwInfrastructure.InitializeForTesting(dwInfrastructure.Type(infra))
	initializeDefaults()
}

func setupClusterApi() {
	fakeclusterapi.Setup(
		fakeclusterapi.WithScheme(NewScheme()),
		fakeclusterapi.WithStatusSubresources(&chev2.CheCluster{}, &appsv1.Deployment{}),
		fakeclusterapi.WithDiscoveryResources(
			&metav1.APIResourceList{
				GroupVersion: "che.eclipse.org/v1alpha1",
				APIResources: []metav1.APIResource{
					{
						Group: "che.eclipse.org",
						Kind:  "KubernetesImagePuller",
					},
				},
			},
			&metav1.APIResourceList{
				GroupVersion: "monitoring.coreos.com/v1",
				APIResources: []metav1.APIResource{
					{
						Group: "monitoring.coreos.com",
						Kind:  "ServiceMonitor",
					},
				},
			},
		),
	)
}

func initializeDefaults() {
	projectRoot, err := utils.FindProjectRoot()
	if err != nil {
		panic("failed to initialize cluster defaults " + err.Error())
	}

	operatorDeployment := &appsv1.Deployment{}
	deploymentFilePath := filepath.Join(projectRoot, "config", "manager", "manager.yaml")

	data, err := os.ReadFile(deploymentFilePath)
	if err != nil {
		panic("failed to initialize cluster defaults " + err.Error())
	}

	err = yaml.Unmarshal(data, operatorDeployment)
	if err != nil {
		panic("failed to initialize cluster defaults " + err.Error())
	}

	for _, container := range operatorDeployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			err = os.Setenv(env.Name, env.Value)
			if err != nil {
				panic("failed to initialize cluster defaults " + err.Error())
			}
		}
	}

	defaults.Initialize()
}
