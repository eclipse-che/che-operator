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
package devworkspace

import (
	"context"
	"errors"

	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DevWorkspaceNamespace      = "devworkspace-controller"
	DevWorkspaceWebhookName    = "controller.devfile.io"
	DevWorkspaceServiceAccount = "devworkspace-controller-serviceaccount"
	DevWorkspaceDeploymentName = "devworkspace-controller-manager"

	DevWorkspaceTemplates                     = "/tmp/devworkspace-operator/templates/deployment/openshift/objects"
	DevWorkspaceServiceAccountFile            = DevWorkspaceTemplates + "/devworkspace-controller-serviceaccount.ServiceAccount.yaml"
	DevWorkspaceRoleFile                      = DevWorkspaceTemplates + "/devworkspace-controller-leader-election-role.Role.yaml"
	DevWorkspaceClusterRoleFile               = DevWorkspaceTemplates + "/devworkspace-controller-role.ClusterRole.yaml"
	DevWorkspaceProxyClusterRoleFile          = DevWorkspaceTemplates + "/devworkspace-controller-proxy-role.ClusterRole.yaml"
	DevWorkspaceViewWorkspacesClusterRoleFile = DevWorkspaceTemplates + "/devworkspace-controller-view-workspaces.ClusterRole.yaml"
	DevWorkspaceEditWorkspacesClusterRoleFile = DevWorkspaceTemplates + "/devworkspace-controller-edit-workspaces.ClusterRole.yaml"
	DevWorkspaceRoleBindingFile               = DevWorkspaceTemplates + "/devworkspace-controller-leader-election-rolebinding.RoleBinding.yaml"
	DevWorkspaceClusterRoleBindingFile        = DevWorkspaceTemplates + "/devworkspace-controller-rolebinding.ClusterRoleBinding.yaml"
	DevWorkspaceProxyClusterRoleBindingFile   = DevWorkspaceTemplates + "/devworkspace-controller-proxy-rolebinding.ClusterRoleBinding.yaml"
	DevWorkspaceWorkspaceRoutingCRDFile       = DevWorkspaceTemplates + "/workspaceroutings.controller.devfile.io.CustomResourceDefinition.yaml"
	DevWorkspaceTemplatesCRDFile              = DevWorkspaceTemplates + "/devworkspacetemplates.workspace.devfile.io.CustomResourceDefinition.yaml"
	DevWorkspaceComponentsCRDFile             = DevWorkspaceTemplates + "/components.controller.devfile.io.CustomResourceDefinition.yaml"
	DevWorkspaceCRDFile                       = DevWorkspaceTemplates + "/devworkspaces.workspace.devfile.io.CustomResourceDefinition.yaml"
	DevWorkspaceConfigMapFile                 = DevWorkspaceTemplates + "/devworkspace-controller-configmap.ConfigMap.yaml"
	DevWorkspaceDeploymentFile                = DevWorkspaceTemplates + "/devworkspace-controller-manager.Deployment.yaml"

	WebTerminalOperatorSubscriptionName = "web-terminal"
	WebTerminalOperatorNamespace        = "openshift-operators"
)

var (
	// cachedObjects
	cachedObj = make(map[string]metav1.Object)
	syncItems = []func(*deploy.DeployContext) (bool, error){
		createNamespace,
		syncServiceAccount,
		syncClusterRole,
		syncProxyClusterRole,
		syncEditWorkspacesClusterRole,
		syncViewWorkspacesClusterRole,
		syncRole,
		syncRoleBinding,
		syncClusterRoleBinding,
		syncProxyClusterRoleBinding,
		syncCRD,
		syncComponentsCRD,
		syncTemplatesCRD,
		syncWorkspaceRoutingCRD,
		syncConfigMap,
		syncDeployment,
	}
)

func ReconcileDevWorkspace(deployContext *deploy.DeployContext) (bool, error) {
	if !util.IsOpenShift4 || !util.IsOAuthEnabled(deployContext.CheCluster) {
		return true, nil
	}

	if !deployContext.CheCluster.Spec.DevWorkspace.Enable {
		return true, nil
	}

	devWorkspaceWebhookExists, err := deploy.IsExists(
		deployContext,
		client.ObjectKey{Name: DevWorkspaceWebhookName},
		&admissionregistrationv1.MutatingWebhookConfiguration{},
	)
	if err != nil {
		return false, err
	}

	if !devWorkspaceWebhookExists {
		for _, syncItem := range syncItems {
			done, err := syncItem(deployContext)
			if !util.IsTestMode() {
				if !done {
					return false, err
				}
			}
		}
	} else {
		if err := checkWebTerminalSubscription(deployContext); err != nil {
			return false, err
		}
	}

	return true, nil
}

func checkWebTerminalSubscription(deployContext *deploy.DeployContext) error {
	subscription := &operatorsv1alpha1.Subscription{}
	if err := deployContext.ClusterAPI.NonCachedClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: WebTerminalOperatorNamespace,
		},
		subscription); err != nil {

		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return errors.New("A non matching version of the Dev Workspace operator is already installed")
}

func createNamespace(deployContext *deploy.DeployContext) (bool, error) {
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
		},
		Spec: corev1.NamespaceSpec{},
	}

	return deploy.CreateIfNotExists(deployContext, namespace)
}

func syncServiceAccount(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceServiceAccountFile, &corev1.ServiceAccount{})
}

func syncRole(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceRoleFile, &rbacv1.Role{})
}

func syncRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceRoleBindingFile, &rbacv1.RoleBinding{})
}

func syncClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{})
}

func syncProxyClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceProxyClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{})
}

func syncClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncProxyClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceProxyClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncViewWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceViewWorkspacesClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncEditWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceEditWorkspacesClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncWorkspaceRoutingCRD(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceWorkspaceRoutingCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncTemplatesCRD(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceTemplatesCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncComponentsCRD(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceComponentsCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncCRD(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncConfigMap(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceConfigMapFile, &corev1.ConfigMap{})
}

func syncDeployment(deployContext *deploy.DeployContext) (bool, error) {
	return syncObject(deployContext, DevWorkspaceDeploymentFile, &appsv1.Deployment{})
}

func syncObject(deployContext *deploy.DeployContext, yamlFile string, obj interface{}) (bool, error) {
	_, exists := cachedObj[yamlFile]
	if !exists {
		if err := util.ReadObject(yamlFile, obj); err != nil {
			return false, err
		}
		cachedObj[yamlFile] = obj.(metav1.Object)
	}

	objectMeta := cachedObj[yamlFile]
	return deploy.CreateIfNotExists(deployContext, objectMeta)
}
