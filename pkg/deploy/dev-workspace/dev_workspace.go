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
	"io/ioutil"
	"os"
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	DevWorkspaceNamespace      = "devworkspace-controller"
	DevWorkspaceWebhookName    = "controller.devfile.io"
	DevWorkspaceServiceAccount = "devworkspace-controller-serviceaccount"
	DevWorkspaceService        = "devworkspace-controller-manager-service"
	DevWorkspaceDeploymentName = "devworkspace-controller-manager"

	SubscriptionResourceName          = "subscriptions"
	ClusterServiceVersionResourceName = "clusterserviceversions"
	DevWorkspaceCSVNamePrefix         = "devworkspace-operator"

	OpenshiftDevWorkspaceTemplatesPath  = "/tmp/devworkspace-operator/templates/deployment/openshift/objects"
	KubernetesDevWorkspaceTemplatesPath = "/tmp/devworkspace-operator/templates/deployment/kubernetes/objects"

	DevWorkspaceTemplates = devWorkspaceTemplatesPath()

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

	// Exits the operator after successful fresh installation of the devworkspace.
	// Can be replaced with something less drastic (especially useful in tests)
	afterInstall = func() {
		logrus.Warn("Exitting the operator after DevWorkspace installation. DevWorkspace support will be initialized on the next start.")
		os.Exit(1)
	}
)

func ReconcileDevWorkspace(deployContext *deploy.DeployContext) (bool, error) {
	if util.IsOpenShift && !util.IsOpenShift4 {
		// OpenShift 3.x is not supported
		return true, nil
	}

	if !deployContext.CheCluster.Spec.DevWorkspace.Enable {
		// Do nothing if Devworkspace is disabled
		return true, nil
	}

	if isDevWorkspaceOperatorCSVExists(deployContext) {
		// Do nothing if Devworkspace has been already deployed via OLM
		return true, nil
	}

	if !deploy.IsDevWorkspaceEngineAllowed() {
		// Note: When the tech-preview-stable-all-namespaces will be by default stable-all-namespaces 7.40.0?, change the channel from the log

		if exists, _ := isDevWorkspaceDeploymentExists(deployContext); !exists {
			// Don't allow to deploy a new Devworkspace operator
			return false, fmt.Errorf("To enable DevWorkspace engine, deploy Eclipse Che from tech-preview channel.")
		}

		// Allow existed Eclipse Che and Devworkspace deployments to work
		// event though is not allowed (for backward compatibility)
		logrus.Warnf("To enable DevWorkspace engine, deploy Eclipse Che from tech-preview channel.")
	}

	if !util.IsOpenShift && util.GetCheServerCustomCheProperty(deployContext.CheCluster, "CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S") != "true" {
		logrus.Warn(`DevWorkspace Che operator can't be enabled on a Kubernetes cluster without explicitly enabled k8s API on che-server. To enable DevWorkspace Che operator set 'spec.server.customCheProperties[CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S]' to 'true'.`)
		return true, nil
	}

	// check if DW exists on the cluster
	devWorkspaceWebhookExists, err := deploy.Get(
		deployContext,
		client.ObjectKey{Name: DevWorkspaceWebhookName},
		&admissionregistrationv1.MutatingWebhookConfiguration{},
	)
	if err != nil {
		return false, err
	}

	if devWorkspaceWebhookExists {
		// if DW exists then check if version matches
		if err := checkWebTerminalSubscription(deployContext); err != nil {
			return false, err
		}
	}

	for _, syncItem := range syncItems {
		_, err := syncItem(deployContext)
		if !util.IsTestMode() {
			if err != nil {
				return false, err
			}
		}
	}

	if !devWorkspaceWebhookExists && !util.IsTestMode() {
		// the webhook did not exist in the cluster
		// this means that we're installing devworkspace and therefore need to restart
		// so that devworkspace support can initialize during the operator startup
		afterInstall()
	}

	return true, nil
}

func isDevWorkspaceDeploymentExists(deployContext *deploy.DeployContext) (bool, error) {
	return deploy.Get(deployContext, types.NamespacedName{
		Namespace: DevWorkspaceNamespace,
		Name:      DevWorkspaceDeploymentName,
	}, &appsv1.Deployment{})
}

func isDevWorkspaceOperatorCSVExists(deployContext *deploy.DeployContext) bool {
	// If clusterserviceversions resource doesn't exist in cluster DWO as well will not be present
	if !util.HasK8SResourceObject(deployContext.ClusterAPI.DiscoveryClient, ClusterServiceVersionResourceName) {
		return false
	}

	csvList := &operatorsv1alpha1.ClusterServiceVersionList{}
	err := deployContext.ClusterAPI.Client.List(context.TODO(), csvList, &client.ListOptions{})
	if err != nil {
		return false
	}

	for _, csv := range csvList.Items {
		if strings.HasPrefix(csv.Name, DevWorkspaceCSVNamePrefix) {
			return true
		}
	}

	return false
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

	return errors.New("a non matching version of the Dev Workspace operator is already installed")
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
	obj2sync, err := readK8SObject(DevWorkspaceConfigMapFile, &corev1.ConfigMap{})
	if err != nil {
		return false, err
	}

	configMap := obj2sync.obj.(*corev1.ConfigMap)
	// Remove when DevWorkspace controller should not care about DWR base host #373 https://github.com/devfile/devworkspace-operator/issues/373
	if !util.IsOpenShift {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string, 1)
		}
		configMap.Data["devworkspace.routing.cluster_host_suffix"] = deployContext.CheCluster.Spec.K8s.IngressDomain
	}

	return syncObject(deployContext, obj2sync, DevWorkspaceNamespace)
}

func syncDwDeployment(deployContext *deploy.DeployContext) (bool, error) {
	obj2sync, err := readK8SObject(DevWorkspaceDeploymentFile, &appsv1.Deployment{})
	if err != nil {
		return false, err
	}

	devworkspaceControllerImage := util.GetValue(deployContext.CheCluster.Spec.DevWorkspace.ControllerImage, deploy.DefaultDevworkspaceControllerImage(deployContext.CheCluster))
	deploymentObject := obj2sync.obj.(*appsv1.Deployment)
	devWorkspaceController := deploymentObject.Spec.Template.Spec.Containers[0]
	devWorkspaceController.Image = devworkspaceControllerImage
	for _, env := range devWorkspaceController.Env {
		if env.Name == "RELATED_IMAGE_devworkspace_webhook_server" {
			env.Value = devworkspaceControllerImage
			break
		}
	}

	return syncObject(deployContext, obj2sync, DevWorkspaceNamespace)
}

func readAndSyncObject(deployContext *deploy.DeployContext, yamlFile string, obj interface{}, namespace string) (bool, error) {
	obj2sync, err := readK8SObject(yamlFile, obj)
	if err != nil {
		return false, err
	}

	return syncObject(deployContext, obj2sync, namespace)
}

func readAndSyncUnstructured(deployContext *deploy.DeployContext, yamlFile string) (bool, error) {
	obj := &unstructured.Unstructured{}
	obj2sync, err := readK8SObject(yamlFile, obj)
	if err != nil {
		return false, err
	}

	return createUnstructured(deployContext, obj2sync.obj.(*unstructured.Unstructured))
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

func syncObject(deployContext *deploy.DeployContext, obj2sync *Object2Sync, namespace string) (bool, error) {
	obj2sync.obj.SetNamespace(namespace)

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

	cheNamespaceOwnership := actual.(metav1.Object).GetAnnotations()[deploy.CheEclipseOrgNamespace]

	// don't sync when object exists but it's not managed by Che
	if exists && cheNamespaceOwnership == "" {
		return true, nil
	}

	// sync objects if it has been created by same operator
	// or it is the only operator with the `spec.devWorkspace.enable: true`
	if !exists ||
		isOnlyOneOperatorManagesDWResources ||
		(cheNamespaceOwnership == deployContext.CheCluster.Namespace &&
			actual.(metav1.Object).GetAnnotations()[deploy.CheEclipseOrgHash256] != obj2sync.hash256) {

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
		data, err := ioutil.ReadFile(yamlFile)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(data, obj)
		if err != nil {
			return nil, err
		}

		cachedObj[yamlFile] = &Object2Sync{
			obj.(metav1.Object),
			util.ComputeHash256(data),
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
