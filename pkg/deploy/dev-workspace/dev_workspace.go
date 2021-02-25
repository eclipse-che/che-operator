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
	"k8s.io/apimachinery/pkg/types"
)

const (
	DevWorkspaceNamespace      = "devworkspace-controller"
	DevWorkspaceWebhookName    = "controller.devfile.io"
	DevWorkspaceServiceAccount = "devworkspace-controller-serviceaccount"

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
	cachedObj = make(map[string]interface{})
	syncItems = []func(*deploy.DeployContext) (bool, error){
		syncNamespace,
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

	if err := checkWebTerminalSubscription(deployContext); err != nil {
		return false, err
	}

	devWorkspaceWebhookExists, err := isDevWorkspaceWebhookExists(deployContext)
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

func isDevWorkspaceWebhookExists(deployContext *deploy.DeployContext) (bool, error) {
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
	if err := deployContext.ClusterAPI.NonCachedClient.Get(
		context.TODO(),
		types.NamespacedName{
			Name: DevWorkspaceWebhookName,
		},
		webhook); err != nil {

		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func syncNamespace(deployContext *deploy.DeployContext) (bool, error) {
	return deploy.CreateNamespace(deployContext, DevWorkspaceNamespace)
}

func syncServiceAccount(deployContext *deploy.DeployContext) (bool, error) {
	sa := &corev1.ServiceAccount{}
	return syncObject(deployContext, DevWorkspaceServiceAccountFile, sa)
}

func syncRole(deployContext *deploy.DeployContext) (bool, error) {
	role := &rbacv1.Role{}
	return syncObject(deployContext, DevWorkspaceRoleFile, role)
}

func syncRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	rb := &rbacv1.RoleBinding{}
	return syncObject(deployContext, DevWorkspaceRoleBindingFile, rb)
}

func syncClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	rb := &rbacv1.ClusterRoleBinding{}
	return syncObject(deployContext, DevWorkspaceClusterRoleBindingFile, rb)
}

func syncProxyClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	rb := &rbacv1.ClusterRoleBinding{}
	return syncObject(deployContext, DevWorkspaceProxyClusterRoleBindingFile, rb)
}

func syncClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	role := &rbacv1.ClusterRole{}
	return syncObject(deployContext, DevWorkspaceClusterRoleFile, role)
}

func syncProxyClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	role := &rbacv1.ClusterRole{}
	return syncObject(deployContext, DevWorkspaceProxyClusterRoleFile, role)
}

func syncViewWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	role := &rbacv1.ClusterRole{}
	return syncObject(deployContext, DevWorkspaceViewWorkspacesClusterRoleFile, role)
}

func syncEditWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	role := &rbacv1.ClusterRole{}
	return syncObject(deployContext, DevWorkspaceEditWorkspacesClusterRoleFile, role)
}

func syncWorkspaceRoutingCRD(deployContext *deploy.DeployContext) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	return syncObject(deployContext, DevWorkspaceWorkspaceRoutingCRDFile, crd)
}

func syncTemplatesCRD(deployContext *deploy.DeployContext) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	return syncObject(deployContext, DevWorkspaceTemplatesCRDFile, crd)
}

func syncComponentsCRD(deployContext *deploy.DeployContext) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	return syncObject(deployContext, DevWorkspaceComponentsCRDFile, crd)
}

func syncCRD(deployContext *deploy.DeployContext) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	return syncObject(deployContext, DevWorkspaceCRDFile, crd)
}

func syncConfigMap(deployContext *deploy.DeployContext) (bool, error) {
	cm := &corev1.ConfigMap{}
	return syncObject(deployContext, DevWorkspaceConfigMapFile, cm)
}

func syncDeployment(deployContext *deploy.DeployContext) (bool, error) {
	deployment := &appsv1.Deployment{}
	return syncObject(deployContext, DevWorkspaceDeploymentFile, deployment)
}

func syncObject(deployContext *deploy.DeployContext, yamlFile string, obj interface{}) (bool, error) {
	_, exists := cachedObj[yamlFile]
	if !exists {
		if err := util.ReadObject(yamlFile, obj); err != nil {
			return false, err
		}
		cachedObj[yamlFile] = obj
	}

	return deploy.Create(deployContext, cachedObj[yamlFile])
}
