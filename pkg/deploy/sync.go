package deploy

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner.
// Returns true if the object was created or updated, false if there was no change detected.
func Sync(deployContext *DeployContext, blueprint metav1.Object, diffOpts cmp.Option) (bool, error) {
	key := client.ObjectKey{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}

	actual, err := Get(deployContext, key, blueprint)
	if err != nil {
		return false, err
	}

	if actual == nil {
		return Create(deployContext, key, blueprint)
	}
	return Update(deployContext, *actual, blueprint, diffOpts)
}

func CreateIfNotExists(deployContext *DeployContext, objectMeta metav1.Object) (bool, error) {
	key := client.ObjectKey{Name: objectMeta.GetName(), Namespace: objectMeta.GetNamespace()}
	exists, err := IsExists(deployContext, key, objectMeta)
	if err != nil {
		return false, err
	}

	if !exists {
		return Create(deployContext, key, objectMeta)
	}

	return true, nil
}

// Indicates if objects exists
func IsExists(deployContext *DeployContext, key client.ObjectKey, objectMeta metav1.Object) (bool, error) {
	actualObject, err := Get(deployContext, key, objectMeta)
	return actualObject != nil, err
}

// Gets object by key
func Get(deployContext *DeployContext, key client.ObjectKey, objectMeta metav1.Object) (*runtime.Object, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return nil, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	client := getClientForObject(objectMeta, deployContext)
	actual := runtimeObject.DeepCopyObject()

	err := client.Get(context.TODO(), key, actual)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &actual, nil
}

func Create(deployContext *DeployContext, key client.ObjectKey, blueprint metav1.Object) (bool, error) {
	blueprintObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", blueprint)
	}

	kind := blueprintObject.GetObjectKind().GroupVersionKind().Kind
	logrus.Infof("Creating a new object: %s, name %s", kind, blueprint.GetName())

	obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
	if err != nil {
		return false, err
	}

	client := getClientForObject(blueprint, deployContext)
	err = client.Create(context.TODO(), obj)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func Delete(deployContext *DeployContext, key client.ObjectKey, objectMeta metav1.Object) (bool, error) {
	runtimeObject, err := Get(deployContext, key, objectMeta)
	if err != nil {
		return false, err
	}

	// object doesn't exist, nothing to delete
	if runtimeObject == nil {
		return true, nil
	}

	client := getClientForObject(objectMeta, deployContext)
	err = client.Delete(context.TODO(), *runtimeObject)
	if err == nil || errors.IsNotFound(err) {
		return true, nil
	}
	return false, err
}

func Update(deployContext *DeployContext, actual runtime.Object, blueprint metav1.Object, diffOpts cmp.Option) (bool, error) {
	client := getClientForObject(blueprint, deployContext)
	actualMeta := actual.(metav1.Object)

	diff := cmp.Diff(actual, blueprint, diffOpts)
	if len(diff) > 0 {
		kind := actual.GetObjectKind().GroupVersionKind().Kind
		logrus.Infof("Updating existing object: %s, name: %s", kind, actualMeta.GetName())
		fmt.Printf("Difference:\n%s", diff)

		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			err := client.Delete(context.TODO(), actual)
			if err != nil {
				return false, err
			}

			obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			err = client.Create(context.TODO(), obj)
			return false, err
		} else {
			obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			obj.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())

			err = client.Update(context.TODO(), obj)
			return false, err
		}
	}
	return true, nil
}

func isUpdateUsingDeleteCreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind
}

func setOwnerReferenceAndConvertToRuntime(deployContext *DeployContext, obj metav1.Object) (runtime.Object, error) {
	robj, ok := obj.(runtime.Object)
	if !ok {
		return nil, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", obj)
	}

	if !shouldSetOwnerReferenceForObject(deployContext, obj) {
		return robj, nil
	}

	err := controllerutil.SetControllerReference(deployContext.CheCluster, obj, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return robj, nil
}

func shouldSetOwnerReferenceForObject(deployContext *DeployContext, obj metav1.Object) bool {
	// empty workspace (cluster scope object) or object in another namespace
	return obj.GetNamespace() == deployContext.CheCluster.Namespace
}

func getClientForObject(objectMeta metav1.Object, deployContext *DeployContext) client.Client {
	// empty namespace (cluster scope object) or object in another namespace
	if deployContext.CheCluster.Namespace == objectMeta.GetNamespace() {
		return deployContext.ClusterAPI.Client
	}
	return deployContext.ClusterAPI.NonCachedClient
}
