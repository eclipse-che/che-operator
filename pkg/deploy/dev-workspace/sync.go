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

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
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

var (
	syncItems = []func(*deploy.DeployContext) (bool, error){
		syncDwService,
		syncDwServiceAccount,
		syncDwClusterRole,
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
		syncDwConfigMap,
		syncDwDeployment,
	}

	DevWorkspaceTemplates = DevWorkspaceTemplatesPath()

	OpenshiftDevWorkspaceTemplatesPath  = "/tmp/devworkspace-operator/templates/deployment/openshift/objects"
	KubernetesDevWorkspaceTemplatesPath = "/tmp/devworkspace-operator/templates/deployment/kubernetes/objects"

	DevWorkspaceServiceAccountFile            = DevWorkspaceTemplates + "/devworkspace-controller-serviceaccount.ServiceAccount.yaml"
	DevWorkspaceRoleFile                      = DevWorkspaceTemplates + "/devworkspace-controller-leader-election-role.Role.yaml"
	DevWorkspaceClusterRoleFile               = DevWorkspaceTemplates + "/devworkspace-controller-role.ClusterRole.yaml"
	DevWorkspaceProxyClusterRoleFile          = DevWorkspaceTemplates + "/devworkspace-controller-proxy-role.ClusterRole.yaml"
	DevWorkspaceViewWorkspacesClusterRoleFile = DevWorkspaceTemplates + "/devworkspace-controller-view-workspaces.ClusterRole.yaml"
	DevWorkspaceEditWorkspacesClusterRoleFile = DevWorkspaceTemplates + "/devworkspace-controller-edit-workspaces.ClusterRole.yaml"
	DevWorkspaceRoleBindingFile               = DevWorkspaceTemplates + "/devworkspace-controller-leader-election-rolebinding.RoleBinding.yaml"
	DevWorkspaceClusterRoleBindingFile        = DevWorkspaceTemplates + "/devworkspace-controller-rolebinding.ClusterRoleBinding.yaml"
	DevWorkspaceProxyClusterRoleBindingFile   = DevWorkspaceTemplates + "/devworkspace-controller-proxy-rolebinding.ClusterRoleBinding.yaml"
	DevWorkspaceWorkspaceRoutingCRDFile       = DevWorkspaceTemplates + "/devworkspaceroutings.controller.devfile.io.CustomResourceDefinition.yaml"
	DevWorkspaceTemplatesCRDFile              = DevWorkspaceTemplates + "/devworkspacetemplates.workspace.devfile.io.CustomResourceDefinition.yaml"
	DevWorkspaceCRDFile                       = DevWorkspaceTemplates + "/devworkspaces.workspace.devfile.io.CustomResourceDefinition.yaml"
	DevWorkspaceConfigMapFile                 = DevWorkspaceTemplates + "/devworkspace-controller-configmap.ConfigMap.yaml"
	DevWorkspaceServiceFile                   = DevWorkspaceTemplates + "/devworkspace-controller-manager-service.Service.yaml"
	DevWorkspaceDeploymentFile                = DevWorkspaceTemplates + "/devworkspace-controller-manager.Deployment.yaml"
	DevWorkspaceIssuerFile                    = DevWorkspaceTemplates + "/devworkspace-controller-selfsigned-issuer.Issuer.yaml"
	DevWorkspaceCertificateFile               = DevWorkspaceTemplates + "/devworkspace-controller-serving-cert.Certificate.yaml"
)

func syncDwServiceAccount(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceServiceAccountFile, &corev1.ServiceAccount{}, DevWorkspaceNamespace)
}

func syncDwService(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceServiceFile, &corev1.Service{}, DevWorkspaceNamespace)
}

func syncDwRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceRoleFile, &rbacv1.Role{}, DevWorkspaceNamespace)
}

func syncDwRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceRoleBindingFile, &rbacv1.RoleBinding{}, DevWorkspaceNamespace)
}

func syncDwClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{}, "")
}

func syncDwProxyClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceProxyClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{}, "")
}

func syncDwClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceClusterRoleFile, &rbacv1.ClusterRole{}, "")
}

func syncDwProxyClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceProxyClusterRoleFile, &rbacv1.ClusterRole{}, "")
}

func syncDwViewWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceViewWorkspacesClusterRoleFile, &rbacv1.ClusterRole{}, "")
}

func syncDwEditWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceEditWorkspacesClusterRoleFile, &rbacv1.ClusterRole{}, "")
}

func syncDwWorkspaceRoutingCRD(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceWorkspaceRoutingCRDFile, &apiextensionsv1.CustomResourceDefinition{}, "")
}

func syncDwTemplatesCRD(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceTemplatesCRDFile, &apiextensionsv1.CustomResourceDefinition{}, "")
}

func syncDwCRD(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCRDFile, &apiextensionsv1.CustomResourceDefinition{}, "")
}

func syncDwIssuer(deployContext *deploy.DeployContext) (bool, error) {
	if !util.IsOpenShift {
		// We're using unstructured to not require a direct dependency on the cert-manager
		// This will cause a failure if cert-manager is not installed, which we're ok with
		// Also, our Sync functionality requires the scheme to have the type we want to persist registered.
		// In case of cert-manager objects, we don't want that because we would have to depend
		// on cert manager, which would require us to also update operator-sdk version because cert-manager
		// uses extension/v1 objects. So, we have to go the unstructured way here...
		return readAndSyncUnstructured(deployContext, DevWorkspaceIssuerFile)
	}
	return true, nil
}

func syncDwCertificate(deployContext *deploy.DeployContext) (bool, error) {
	if !util.IsOpenShift {
		// We're using unstructured to not require a direct dependency on the cert-manager
		// This will cause a failure if cert-manager is not installed, which we're ok with
		// Also, our Sync functionality requires the scheme to have the type we want to persist registered.
		// In case of cert-manager objects, we don't want that because we would have to depend
		// on cert manager, which would require us to also update operator-sdk version because cert-manager
		// uses extension/v1 objects. So, we have to go the unstructured way here...
		return readAndSyncUnstructured(deployContext, DevWorkspaceCertificateFile)
	}
	return true, nil
}

func syncDwConfigMap(deployContext *deploy.DeployContext) (bool, error) {
	configMap := &corev1.ConfigMap{}
	err := readK8SObject(DevWorkspaceConfigMapFile, configMap)
	if err != nil {
		return false, err
	}
	return syncObject(deployContext, configMap, DevWorkspaceNamespace)
}

func syncDwDeployment(deployContext *deploy.DeployContext) (bool, error) {
	deployment := &appsv1.Deployment{}
	err := readK8SObject(DevWorkspaceDeploymentFile, deployment)
	if err != nil {
		return false, err
	}

	devworkspaceControllerImage := util.GetValue(deployContext.CheCluster.Spec.DevWorkspace.ControllerImage, deploy.DefaultDevworkspaceControllerImage(deployContext.CheCluster))
	devWorkspaceController := deployment.Spec.Template.Spec.Containers[0]
	devWorkspaceController.Image = devworkspaceControllerImage
	for _, env := range devWorkspaceController.Env {
		if env.Name == "RELATED_IMAGE_devworkspace_webhook_server" {
			env.Value = devworkspaceControllerImage
			break
		}
	}

	return syncObject(deployContext, deployment, DevWorkspaceNamespace)
}

func readAndSyncObject(deployContext *deploy.DeployContext, yamlFile string, obj client.Object, namespace string) (bool, error) {
	err := readK8SObject(yamlFile, obj)
	if err != nil {
		return false, err
	}

	return syncObject(deployContext, obj, namespace)
}

func readAndSyncUnstructured(deployContext *deploy.DeployContext, yamlFile string) (bool, error) {
	obj := &unstructured.Unstructured{}
	err := readK8SUnstructured(yamlFile, obj)
	if err != nil {
		return false, err
	}

	return createUnstructured(deployContext, obj)
}

func createUnstructured(deployContext *deploy.DeployContext, obj *unstructured.Unstructured) (bool, error) {
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

func syncObject(deployContext *deploy.DeployContext, obj2sync client.Object, namespace string) (bool, error) {
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
	if !exists || actual.(metav1.Object).GetAnnotations()[deploy.CheEclipseOrgHash256] != obj2sync.GetAnnotations()[deploy.CheEclipseOrgHash256] {
		obj2sync.GetAnnotations()[deploy.CheEclipseOrgNamespace] = deployContext.CheCluster.Namespace
		return deploy.Sync(deployContext, obj2sync)
	}

	return true, nil
}

func DevWorkspaceTemplatesPath() string {
	if util.IsOpenShift {
		return OpenshiftDevWorkspaceTemplatesPath
	}
	return KubernetesDevWorkspaceTemplatesPath
}
