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
package devfileregistry

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/registry"
	"github.com/eclipse-che/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

func (d *DevfileRegistryReconciler) getDevfileRegistryDeploymentSpec(ctx *deploy.DeployContext) *appsv1.Deployment {
	registryType := "devfile"
	registryImage := util.GetValue(ctx.CheCluster.Spec.Server.DevfileRegistryImage, deploy.DefaultDevfileRegistryImage(ctx.CheCluster))
	registryImagePullPolicy := v1.PullPolicy(util.GetValue(string(ctx.CheCluster.Spec.Server.DevfileRegistryPullPolicy), deploy.DefaultPullPolicyFromDockerImage(registryImage)))
	probePath := "/devfiles/"
	devfileImagesEnv := util.GetEnvByRegExp("^.*devfile_registry_image.*$")

	// If there is a devfile registry deployed by operator
	if !ctx.CheCluster.Spec.Server.ExternalDevfileRegistry {
		devfileImagesEnv = append(devfileImagesEnv,
			corev1.EnvVar{
				Name:  "CHE_DEVFILE_REGISTRY_INTERNAL_URL",
				Value: fmt.Sprintf("http://%s.%s.svc:8080", deploy.DevfileRegistryName, ctx.CheCluster.Namespace)},
		)
	}

	resources := v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceMemory: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.DevfileRegistryMemoryRequest,
				deploy.DefaultDevfileRegistryMemoryRequest),
			v1.ResourceCPU: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.DevfileRegistryCpuRequest,
				deploy.DefaultDevfileRegistryCpuRequest),
		},
		Limits: v1.ResourceList{
			v1.ResourceMemory: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.DevfileRegistryMemoryLimit,
				deploy.DefaultDevfileRegistryMemoryLimit),
			v1.ResourceCPU: util.GetResourceQuantity(
				ctx.CheCluster.Spec.Server.DevfileRegistryCpuLimit,
				deploy.DefaultDevfileRegistryCpuLimit),
		},
	}

	return registry.GetSpecRegistryDeployment(
		ctx,
		registryType,
		registryImage,
		devfileImagesEnv,
		registryImagePullPolicy,
		resources,
		probePath)
}
