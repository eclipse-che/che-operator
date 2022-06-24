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
package devworkspace

import (
	"context"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OpenshiftDevWorkspaceTemplatesPath  = "/tmp/devworkspace-operator/templates/deployment/openshift/objects"
	KubernetesDevWorkspaceTemplatesPath = "/tmp/devworkspace-operator/templates/deployment/kubernetes/objects"
)

var (
	syncItems = []func(*chetypes.DeployContext) (bool, error){
		syncDwService,
		syncDwMetricService,
		syncDwServiceAccount,
		syncDwClusterRole,
		syncDwMetricsClusterRole,
		syncDwProxyClusterRole,
		syncDwEditWorkspacesClusterRole,
		syncDwViewWorkspacesClusterRole,
		syncDwRole,
		syncDwRoleBinding,
		syncDwClusterRoleBinding,
		syncDwProxyClusterRoleBinding,
		syncDwIssuer,
		syncDwCertificate,
		syncDwCRD,
		syncDwTemplatesCRD,
		syncDwWorkspaceRoutingCRD,
		syncDwConfigCRD,
		syncDwDeployment,
	}
)

func syncDwServiceAccount(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-serviceaccount.ServiceAccount.yaml"
	return readAndSyncObject(deployContext, path, &corev1.ServiceAccount{}, DevWorkspaceNamespace)
}

func syncDwService(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-manager-service.Service.yaml"
	return readAndSyncObject(deployContext, path, &corev1.Service{}, DevWorkspaceNamespace)
}

func syncDwMetricService(deployContext *chetypes.DeployContext) (bool, error) {
	filePath := devWorkspaceTemplatesPath() + "/devworkspace-controller-metrics.Service.yaml"
	return readAndSyncObject(deployContext, filePath, &corev1.Service{}, DevWorkspaceNamespace)
}

func syncDwRole(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-leader-election-role.Role.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.Role{}, DevWorkspaceNamespace)
}

func syncDwRoleBinding(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-leader-election-rolebinding.RoleBinding.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.RoleBinding{}, DevWorkspaceNamespace)
}

func syncDwClusterRoleBinding(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-rolebinding.ClusterRoleBinding.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.ClusterRoleBinding{}, "")
}

func syncDwProxyClusterRoleBinding(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-proxy-rolebinding.ClusterRoleBinding.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.ClusterRoleBinding{}, "")
}

func syncDwClusterRole(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-role.ClusterRole.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.ClusterRole{}, "")
}

func syncDwMetricsClusterRole(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-metrics-reader.ClusterRole.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.ClusterRole{}, "")
}

func syncDwProxyClusterRole(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-proxy-role.ClusterRole.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.ClusterRole{}, "")
}

func syncDwViewWorkspacesClusterRole(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-view-workspaces.ClusterRole.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.ClusterRole{}, "")
}

func syncDwEditWorkspacesClusterRole(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-edit-workspaces.ClusterRole.yaml"
	return readAndSyncObject(deployContext, path, &rbacv1.ClusterRole{}, "")
}

func syncDwWorkspaceRoutingCRD(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspaceroutings.controller.devfile.io.CustomResourceDefinition.yaml"
	return readAndSyncObject(deployContext, path, &apiextensionsv1.CustomResourceDefinition{}, "")
}

func syncDwTemplatesCRD(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspacetemplates.workspace.devfile.io.CustomResourceDefinition.yaml"
	return readAndSyncObject(deployContext, path, &apiextensionsv1.CustomResourceDefinition{}, "")
}

func syncDwCRD(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspaces.workspace.devfile.io.CustomResourceDefinition.yaml"
	return readAndSyncObject(deployContext, path, &apiextensionsv1.CustomResourceDefinition{}, "")
}

func syncDwConfigCRD(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspaceoperatorconfigs.controller.devfile.io.CustomResourceDefinition.yaml"
	return readAndSyncObject(deployContext, path, &apiextensionsv1.CustomResourceDefinition{}, "")
}

func syncDwIssuer(deployContext *chetypes.DeployContext) (bool, error) {
	if !infrastructure.IsOpenShift() {
		// We're using unstructured to not require a direct dependency on the cert-manager
		// This will cause a failure if cert-manager is not installed, which we're ok with
		// Also, our Sync functionality requires the scheme to have the type we want to persist registered.
		// In case of cert-manager objects, we don't want that because we would have to depend
		// on cert manager, which would require us to also update operator-sdk version because cert-manager
		// uses extension/v1 objects. So, we have to go the unstructured way here...
		path := devWorkspaceTemplatesPath() + "/devworkspace-controller-selfsigned-issuer.Issuer.yaml"
		return readAndSyncUnstructured(deployContext, path)
	}
	return true, nil
}

func syncDwCertificate(deployContext *chetypes.DeployContext) (bool, error) {
	if !infrastructure.IsOpenShift() {
		// We're using unstructured to not require a direct dependency on the cert-manager
		// This will cause a failure if cert-manager is not installed, which we're ok with
		// Also, our Sync functionality requires the scheme to have the type we want to persist registered.
		// In case of cert-manager objects, we don't want that because we would have to depend
		// on cert manager, which would require us to also update operator-sdk version because cert-manager
		// uses extension/v1 objects. So, we have to go the unstructured way here...
		path := devWorkspaceTemplatesPath() + "/devworkspace-controller-serving-cert.Certificate.yaml"
		return readAndSyncUnstructured(deployContext, path)
	}
	return true, nil
}

func syncDwDeployment(deployContext *chetypes.DeployContext) (bool, error) {
	path := devWorkspaceTemplatesPath() + "/devworkspace-controller-manager.Deployment.yaml"
	deployment := &appsv1.Deployment{}
	err := readK8SObject(path, deployment)
	if err != nil {
		return false, err
	}

	devworkspaceControllerImage := defaults.GetDevworkspaceControllerImage(deployContext.CheCluster)
	if deployContext.CheCluster.Spec.Components.DevWorkspace.Deployment != nil {
		if len(deployContext.CheCluster.Spec.Components.DevWorkspace.Deployment.Containers) != 0 {
			devworkspaceControllerImage = utils.GetValue(deployContext.CheCluster.Spec.Components.DevWorkspace.Deployment.Containers[0].Image, devworkspaceControllerImage)
		}
	}

	for contIdx, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "devworkspace-controller" {
			deployment.Spec.Template.Spec.Containers[contIdx].Image = devworkspaceControllerImage
		} else {
			deployment.Spec.Template.Spec.Containers[contIdx].Image = defaults.PatchDefaultImageName(
				deployContext.CheCluster,
				deployment.Spec.Template.Spec.Containers[contIdx].Image,
			)
		}

		for envIdx, env := range container.Env {
			if env.Name == "RELATED_IMAGE_devworkspace_webhook_server" {
				deployment.Spec.Template.Spec.Containers[contIdx].Env[envIdx].Value = devworkspaceControllerImage
			} else if strings.HasPrefix(env.Name, "RELATED_IMAGE_") {
				deployment.Spec.Template.Spec.Containers[contIdx].Env[envIdx].Value = defaults.PatchDefaultImageName(deployContext.CheCluster, env.Value)
			}
		}
	}

	return syncObject(deployContext, deployment, DevWorkspaceNamespace)
}

func readAndSyncObject(deployContext *chetypes.DeployContext, yamlFile string, obj client.Object, namespace string) (bool, error) {
	err := readK8SObject(yamlFile, obj)
	if err != nil {
		return false, err
	}

	return syncObject(deployContext, obj, namespace)
}

func readAndSyncUnstructured(deployContext *chetypes.DeployContext, yamlFile string) (bool, error) {
	obj := &unstructured.Unstructured{}
	err := readK8SUnstructured(yamlFile, obj)
	if err != nil {
		return false, err
	}

	return createUnstructured(deployContext, obj)
}

func createUnstructured(deployContext *chetypes.DeployContext, obj *unstructured.Unstructured) (bool, error) {
	check := &unstructured.Unstructured{}
	check.SetGroupVersionKind(obj.GroupVersionKind())

	err := deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			check = nil
		} else {
			return false, err
		}
	}

	if check == nil {
		err = deployContext.ClusterAPI.Client.Create(context.TODO(), obj)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return false, nil
			}
			return false, err
		}
	}

	return true, nil
}

func syncObject(deployContext *chetypes.DeployContext, obj2sync client.Object, namespace string) (bool, error) {
	obj2sync.SetNamespace(namespace)

	actual, err := deployContext.ClusterAPI.Scheme.New(obj2sync.GetObjectKind().GroupVersionKind())
	if err != nil {
		return false, err
	}

	key := types.NamespacedName{Namespace: obj2sync.GetNamespace(), Name: obj2sync.GetName()}
	exists, err := deploy.Get(deployContext, key, actual.(client.Object))
	if err != nil {
		return false, err
	}

	// sync objects if it does not exists or has outdated hash
	if !exists || actual.(metav1.Object).GetAnnotations()[constants.CheEclipseOrgHash256] != obj2sync.GetAnnotations()[constants.CheEclipseOrgHash256] {
		obj2sync.GetAnnotations()[constants.CheEclipseOrgNamespace] = deployContext.CheCluster.Namespace
		return deploy.Sync(deployContext, obj2sync)
	}

	return true, nil
}

func devWorkspaceTemplatesPath() string {
	if infrastructure.IsOpenShift() {
		return OpenshiftDevWorkspaceTemplatesPath
	}
	return KubernetesDevWorkspaceTemplatesPath
}
