package devworkspace

import (
	"context"
	"fmt"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"io/ioutil"

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

	// sync objects if it does not exists or has outdated hash
	if !exists || actual.(metav1.Object).GetAnnotations()[deploy.CheEclipseOrgHash256] != obj2sync.hash256 {
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
