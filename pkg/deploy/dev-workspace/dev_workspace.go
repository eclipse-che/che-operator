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
	"fmt"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	DevWorkspaceNamespace      = "devworkspace-controller"
	DevWorkspaceCheNamespace   = "devworkspace-che"
	DevWorkspaceWebhookName    = "controller.devfile.io"
	DevWorkspaceServiceAccount = "devworkspace-controller-serviceaccount"
	DevWorkspaceDeploymentName = "devworkspace-controller-manager"
	SubscriptionResourceName   = "subscriptions"
	CheManagerResourcename     = "chemanagers"

	OpenshiftDevWorkspaceTemplatesPath     = "/tmp/devworkspace-operator/templates/deployment/openshift/objects"
	OpenshiftDevWorkspaceCheTemplatesPath  = "/tmp/devworkspace-che-operator/templates/deployment/openshift/objects/"
	KubernetesDevWorkspaceTemplatesPath    = "/tmp/devworkspace-operator/templates/deployment/kubernetes/objects"
	KubernetesDevWorkspaceCheTemplatesPath = "/tmp/devworkspace-che-operator/templates/deployment/kubernetes/objects/"

	DevWorkspaceTemplates    = devWorkspaceTemplatesPath()
	DevWorkspaceCheTemplates = devWorkspaceCheTemplatesPath()

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
	DevWorkspaceDeploymentFile                = DevWorkspaceTemplates + "/devworkspace-controller-manager.Deployment.yaml"

	DevWorkspaceCheServiceAccountFile           = DevWorkspaceCheTemplates + "/devworkspace-che-serviceaccount.ServiceAccount.yaml"
	DevWorkspaceCheRoleFile                     = DevWorkspaceCheTemplates + "/devworkspace-che-leader-election-role.Role.yaml"
	DevWorkspaceCheClusterRoleFile              = DevWorkspaceCheTemplates + "/devworkspace-che-role.ClusterRole.yaml"
	DevWorkspaceCheProxyClusterRoleFile         = DevWorkspaceCheTemplates + "/devworkspace-che-proxy-role.ClusterRole.yaml"
	DevWorkspaceCheMetricsReaderClusterRoleFile = DevWorkspaceCheTemplates + "/devworkspace-che-metrics-reader.ClusterRole.yaml"
	DevWorkspaceCheRoleBindingFile              = DevWorkspaceCheTemplates + "/devworkspace-che-leader-election-rolebinding.RoleBinding.yaml"
	DevWorkspaceCheClusterRoleBindingFile       = DevWorkspaceCheTemplates + "/devworkspace-che-rolebinding.ClusterRoleBinding.yaml"
	DevWorkspaceCheProxyClusterRoleBindingFile  = DevWorkspaceCheTemplates + "/devworkspace-che-proxy-rolebinding.ClusterRoleBinding.yaml"
	DevWorkspaceCheManagersCRDFile              = DevWorkspaceCheTemplates + "/chemanagers.che.eclipse.org.CustomResourceDefinition.yaml"
	DevWorkspaceCheConfigMapFile                = DevWorkspaceCheTemplates + "/devworkspace-che-configmap.ConfigMap.yaml"
	DevWorkspaceCheDeploymentFile               = DevWorkspaceCheTemplates + "/devworkspace-che-manager.Deployment.yaml"
	DevWorkspaceCheMetricsServiceFile           = DevWorkspaceCheTemplates + "/devworkspace-che-controller-manager-metrics-service.Service.yaml"

	WebTerminalOperatorSubscriptionName = "web-terminal"
	WebTerminalOperatorNamespace        = "openshift-operators"
)

type Object2Sync struct {
	obj     metav1.Object
	hash256 string
}

var (
	// cachedObjects
	cachedObj = make(map[string]*Object2Sync)
	syncItems = []func(*deploy.DeployContext) (bool, error){
		createDwNamespace,
		syncDwServiceAccount,
		syncDwClusterRole,
		syncDwProxyClusterRole,
		syncDwEditWorkspacesClusterRole,
		syncDwViewWorkspacesClusterRole,
		syncDwRole,
		syncDwRoleBinding,
		syncDwClusterRoleBinding,
		syncDwProxyClusterRoleBinding,
		syncDwCRD,
		syncDwTemplatesCRD,
		syncDwWorkspaceRoutingCRD,
		syncDwConfigMap,
		syncDwDeployment,
	}

	syncDwCheItems = []func(*deploy.DeployContext) (bool, error){
		createDwCheNamespace,
		syncDwCheServiceAccount,
		syncDwCheClusterRole,
		syncDwCheProxyClusterRole,
		syncDwCheMetricsClusterRole,
		syncDwCheLeaderRole,
		syncDwCheLeaderRoleBinding,
		syncDwCheProxyRoleBinding,
		syncDwCheRoleBinding,
		syncDwCheCRD,
		synDwCheCR,
		syncDwCheConfigMap,
		syncDwCheMetricsService,
		synDwCheDeployment,
	}
)

func ReconcileDevWorkspace(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.Spec.DevWorkspace.Enable {
		return true, nil
	}

	devWorkspaceWebhookExists, err := deploy.Get(
		deployContext,
		client.ObjectKey{Name: DevWorkspaceWebhookName},
		&admissionregistrationv1.MutatingWebhookConfiguration{},
	)
	if err != nil {
		return false, err
	}

	if devWorkspaceWebhookExists {
		if err := checkWebTerminalSubscription(deployContext); err != nil {
			return false, err
		}
	}

	for _, syncItem := range syncItems {
		done, err := syncItem(deployContext)
		if !util.IsTestMode() {
			if !done {
				return false, err
			}
		}
	}

	for _, syncItem := range syncDwCheItems {
		done, err := syncItem(deployContext)
		if !util.IsTestMode() {
			if !done {
				return false, err
			}
		}
	}

	return true, nil
}

func checkWebTerminalSubscription(deployContext *deploy.DeployContext) error {
	// If subscriptions resource doesn't exist in cluster WTO as well will not be present
	if !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, SubscriptionResourceName) {
		return nil
	}

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

func createDwNamespace(deployContext *deploy.DeployContext) (bool, error) {
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

func syncDwServiceAccount(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceServiceAccountFile, &corev1.ServiceAccount{})
}

func syncDwRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceRoleFile, &rbacv1.Role{})
}

func syncDwRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceRoleBindingFile, &rbacv1.RoleBinding{})
}

func syncDwClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{})
}

func syncDwProxyClusterRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceProxyClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{})
}

func syncDwClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncDwProxyClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceProxyClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncDwViewWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceViewWorkspacesClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncDwEditWorkspacesClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceEditWorkspacesClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncDwWorkspaceRoutingCRD(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceWorkspaceRoutingCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncDwTemplatesCRD(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceTemplatesCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncDwCRD(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncDwConfigMap(deployContext *deploy.DeployContext) (bool, error) {
	devObject, err := readK8SObject(DevWorkspaceConfigMapFile, &corev1.ConfigMap{})
	if err != nil {
		return false, err
	}

	configMap := devObject.obj.(*corev1.ConfigMap)
	// Remove when DevWorkspace controller should not care about DWR base host #373 https://github.com/devfile/devworkspace-operator/issues/373
	if !util.IsOpenShift {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string, 1)
		}
		configMap.Data["devworkspace.routing.cluster_host_suffix"] = deployContext.CheCluster.Spec.K8s.IngressDomain
	}

	return syncObject(deployContext, devObject)
}

func syncDwDeployment(deployContext *deploy.DeployContext) (bool, error) {
	devObject, err := readK8SObject(DevWorkspaceDeploymentFile, &appsv1.Deployment{})
	if err != nil {
		return false, err
	}

	devworkspaceControllerImage := util.GetValue(deployContext.CheCluster.Spec.DevWorkspace.ControllerImage, deploy.DefaultDevworkspaceControllerImage(deployContext.CheCluster))
	deploymentObject := devObject.obj.(*appsv1.Deployment)
	deploymentObject.Spec.Template.Spec.Containers[0].Image = devworkspaceControllerImage

	return syncObject(deployContext, devObject)
}

func createDwCheNamespace(deployContext *deploy.DeployContext) (bool, error) {
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceCheNamespace,
		},
		Spec: corev1.NamespaceSpec{},
	}

	return deploy.CreateIfNotExists(deployContext, namespace)
}

func syncDwCheServiceAccount(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheServiceAccountFile, &corev1.ServiceAccount{})
}

func syncDwCheClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncDwCheProxyClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheProxyClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncDwCheMetricsClusterRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheMetricsReaderClusterRoleFile, &rbacv1.ClusterRole{})
}

func syncDwCheLeaderRole(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheRoleFile, &rbacv1.Role{})
}

func syncDwCheLeaderRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheRoleBindingFile, &rbacv1.RoleBinding{})
}

func syncDwCheProxyRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheProxyClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{})
}

func syncDwCheRoleBinding(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheClusterRoleBindingFile, &rbacv1.ClusterRoleBinding{})
}

func syncDwCheCRD(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheManagersCRDFile, &apiextensionsv1.CustomResourceDefinition{})
}

func syncDwCheConfigMap(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheConfigMapFile, &corev1.ConfigMap{})
}

func syncDwCheMetricsService(deployContext *deploy.DeployContext) (bool, error) {
	return readAndSyncObject(deployContext, DevWorkspaceCheMetricsServiceFile, &corev1.Service{})
}

func synDwCheCR(deployContext *deploy.DeployContext) (bool, error) {
	// We want to create a default CheManager instance to be able to configure the che-specific
	// parts of the installation, but at the same time we don't want to add a dependency on
	// devworkspace-che-operator. Note that this way of initializing will probably see changes
	// once we figure out https://github.com/eclipse/che/issues/19220

	// Wait until CRD for CheManager is created
	if !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, CheManagerResourcename) {
		return false, nil
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "che.eclipse.org", Version: "v1alpha1", Kind: "CheManager"})
	err := deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "devworkspace-che", Namespace: DevWorkspaceCheNamespace}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			obj = nil
		} else {
			return false, err
		}
	}

	if obj == nil {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "che.eclipse.org",
			Version: "v1alpha1",
			Kind:    "CheManager",
		})
		obj.SetName("devworkspace-che")
		obj.SetNamespace(DevWorkspaceCheNamespace)

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

func synDwCheDeployment(deployContext *deploy.DeployContext) (bool, error) {
	devObject, err := readK8SObject(DevWorkspaceCheDeploymentFile, &appsv1.Deployment{})
	if err != nil {
		return false, err
	}

	devworkspaceCheOperatorImage := deploy.DefaultDevworkspaceCheOperatorImage(deployContext.CheCluster)
	deploymentObject := devObject.obj.(*appsv1.Deployment)
	deploymentObject.Spec.Template.Spec.Containers[0].Image = devworkspaceCheOperatorImage

	return syncObject(deployContext, devObject)
}

func readAndSyncObject(deployContext *deploy.DeployContext, yamlFile string, obj interface{}) (bool, error) {
	obj2sync, err := readK8SObject(yamlFile, obj)
	if err != nil {
		return false, err
	}

	return syncObject(deployContext, obj2sync)
}

func syncObject(deployContext *deploy.DeployContext, obj2sync *Object2Sync) (bool, error) {
	runtimeObject, ok := obj2sync.obj.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject()
	key := types.NamespacedName{Namespace: obj2sync.obj.GetNamespace(), Name: obj2sync.obj.GetName()}
	exists, err := deploy.Get(deployContext, key, actual.(metav1.Object))
	if err != nil {
		return false, err
	}

	isOnlyOneOperatorManagesDWResources, err := isOnlyOneOperatorManagesDWResources(deployContext)
	if err != nil {
		return false, err
	}

	// sync objects if it has been created by same operator
	// or it is the only operator with the `spec.devWorkspace.enable: true`
	if !exists ||
		(actual.(metav1.Object).GetAnnotations()[deploy.CheEclipseOrgHash256] != obj2sync.hash256 &&
			(actual.(metav1.Object).GetAnnotations()[deploy.CheEclipseOrgNamespace] == deployContext.CheCluster.Namespace || isOnlyOneOperatorManagesDWResources)) {

		setAnnotations(deployContext, obj2sync)
		return deploy.Sync(deployContext, obj2sync.obj)
	}

	return true, nil
}

func setAnnotations(deployContext *deploy.DeployContext, obj2sync *Object2Sync) {
	annotations := obj2sync.obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[deploy.CheEclipseOrgNamespace] = deployContext.CheCluster.Namespace
	annotations[deploy.CheEclipseOrgHash256] = obj2sync.hash256
	obj2sync.obj.SetAnnotations(annotations)
}

func isOnlyOneOperatorManagesDWResources(deployContext *deploy.DeployContext) (bool, error) {
	cheClusters := &orgv1.CheClusterList{}
	err := deployContext.ClusterAPI.NonCachedClient.List(context.TODO(), cheClusters)
	if err != nil {
		return false, err
	}

	devWorkspaceEnabledNum := 0
	for _, cheCluster := range cheClusters.Items {
		if cheCluster.Spec.DevWorkspace.Enable {
			devWorkspaceEnabledNum++
		}
	}

	return devWorkspaceEnabledNum == 1, nil
}

func readK8SObject(yamlFile string, obj interface{}) (*Object2Sync, error) {
	_, exists := cachedObj[yamlFile]
	if !exists {
		if err := util.ReadObject(yamlFile, obj); err != nil {
			return nil, err
		}

		hash256, err := util.ComputeHash256(yamlFile)
		if err != nil {
			return nil, err
		}

		cachedObj[yamlFile] = &Object2Sync{
			obj.(metav1.Object),
			hash256,
		}
	}

	return cachedObj[yamlFile], nil
}

func devWorkspaceTemplatesPath() string {
	if util.IsOpenShift {
		return OpenshiftDevWorkspaceTemplatesPath
	}
	return KubernetesDevWorkspaceTemplatesPath
}

func devWorkspaceCheTemplatesPath() string {
	if util.IsOpenShift {
		return OpenshiftDevWorkspaceCheTemplatesPath
	}
	return KubernetesDevWorkspaceCheTemplatesPath
}
