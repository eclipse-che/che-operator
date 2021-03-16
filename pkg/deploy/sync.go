package deploy

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner.
// Returns true if object is up to date otherwiser returns false
func Sync(deployContext *DeployContext, blueprint metav1.Object, diffOpts cmp.Option) (bool, error) {
	key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}

	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject()
	client := getClientForObject(blueprint, deployContext)
	exists, err := doGet(client, key, actual)
	if err != nil {
		return false, err
	}

	if !exists {
		return Create(deployContext, blueprint)
	}
	return Update(deployContext, actual, blueprint, diffOpts)
}

func SyncWithFinalizer(
	deployContext *DeployContext,
	blueprint metav1.Object,
	diffOpts cmp.Option,
	finalizer string) (bool, error) {

	if deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		done, err := Sync(deployContext, blueprint, crbDiffOpts)
		if !done {
			return done, err
		}
		err = AppendFinalizer(deployContext, finalizer)
		return err == nil, err
	} else {
		key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}
		err := DeleteObjectAndFinalizer(deployContext, key, blueprint, finalizer)
		return err == nil, err
	}
}

// Gets object by key.
// Returns true if object exists otherwise returns false.
func Get(deployContext *DeployContext, key client.ObjectKey, actual metav1.Object) (bool, error) {
	runtimeObject, ok := actual.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	client := getClientForObject(actual, deployContext)
	return doGet(client, key, runtimeObject)
}

// Creates object.
// Return true if a new object is created or has been already created otherwise returns false.
func CreateIfNotExists(deployContext *DeployContext, blueprint metav1.Object) (bool, error) {
	client := getClientForObject(blueprint, deployContext)
	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject()
	key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}
	exists, err := doGet(client, key, actual)
	if err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	kind := runtimeObject.GetObjectKind().GroupVersionKind().Kind
	logrus.Infof("Creating a new object: %s, name: %s", kind, blueprint.GetName())

	err = setOwnerReferenceIfNeeded(deployContext, blueprint)
	if err != nil {
		return false, err
	}

	return doCreate(client, runtimeObject, true)
}

// Creates object.
// Return true if a new object is created otherwise returns false.
func Create(deployContext *DeployContext, blueprint metav1.Object) (bool, error) {
	client := getClientForObject(blueprint, deployContext)
	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	kind := runtimeObject.GetObjectKind().GroupVersionKind().Kind
	logrus.Infof("Creating a new object: %s, name: %s", kind, blueprint.GetName())

	err := setOwnerReferenceIfNeeded(deployContext, blueprint)
	if err != nil {
		return false, err
	}

	return doCreate(client, runtimeObject, false)
}

// Deletes object.
// Returns true if object deleted or not found otherwise returns false.
func Delete(deployContext *DeployContext, key client.ObjectKey, blueprint metav1.Object) (bool, error) {
	client := getClientForObject(blueprint, deployContext)
	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject()
	exists, err := doGet(client, key, actual)
	if err != nil {
		return false, err
	} else if !exists {
		return true, nil
	}

	kind := runtimeObject.GetObjectKind().GroupVersionKind().Kind
	logrus.Infof("Deleting object: %s, name: %s", kind, key.Name)

	return doDelete(client, actual)
}

// Updates object.
// Returns true if object is up to date otherwiser return false
func Update(deployContext *DeployContext, actual runtime.Object, blueprint metav1.Object, diffOpts cmp.Option) (bool, error) {
	actualMeta := actual.(metav1.Object)

	diff := cmp.Diff(blueprint, actual, diffOpts)
	if len(diff) > 0 {
		kind := actual.GetObjectKind().GroupVersionKind().Kind
		logrus.Infof("Updating existing object: %s, name: %s", kind, actualMeta.GetName())
		fmt.Printf("Difference:\n%s", diff)

		client := getClientForObject(blueprint, deployContext)
		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			done, err := doDelete(client, actual)
			if !done {
				return false, err
			}

			err = setOwnerReferenceIfNeeded(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			return doCreate(client, blueprint.(runtime.Object), false)
		} else {
			err := setOwnerReferenceIfNeeded(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			obj, ok := blueprint.(runtime.Object)
			if !ok {
				return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", obj)
			}

			// to be able to update, we need to set the resource version of the object that we know of
			obj.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())
			return doUpdate(client, obj)
		}
	}
	return true, nil
}

func doCreate(client client.Client, object runtime.Object, returnTrueIfAlreadyExists bool) (bool, error) {
	err := client.Create(context.TODO(), object)
	if err == nil {
		return true, nil
	} else if errors.IsAlreadyExists(err) {
		return returnTrueIfAlreadyExists, nil
	} else {
		return false, err
	}
}

func doDelete(client client.Client, object runtime.Object) (bool, error) {
	err := client.Delete(context.TODO(), object)
	if err == nil || errors.IsNotFound(err) {
		return true, nil
	} else {
		return false, err
	}
}

func doUpdate(client client.Client, object runtime.Object) (bool, error) {
	err := client.Update(context.TODO(), object)
	if err == nil {
		return true, nil
	} else {
		return false, err
	}
}

func doGet(client client.Client, key client.ObjectKey, object runtime.Object) (bool, error) {
	err := client.Get(context.TODO(), key, object)
	if err == nil {
		return true, nil
	} else if errors.IsNotFound(err) {
		return false, nil
	} else {
		return false, err
	}
}

func isUpdateUsingDeleteCreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind
}

func setOwnerReferenceIfNeeded(deployContext *DeployContext, blueprint metav1.Object) error {
	if shouldSetOwnerReferenceForObject(deployContext, blueprint) {
		return controllerutil.SetControllerReference(deployContext.CheCluster, blueprint, deployContext.ClusterAPI.Scheme)
	}

	return nil
}

func shouldSetOwnerReferenceForObject(deployContext *DeployContext, blueprint metav1.Object) bool {
	// empty workspace (cluster scope object) or object in another namespace
	return blueprint.GetNamespace() == deployContext.CheCluster.Namespace
}

func getClientForObject(objectMeta metav1.Object, deployContext *DeployContext) client.Client {
	// empty namespace (cluster scope object) or object in another namespace
	if deployContext.CheCluster.Namespace == objectMeta.GetNamespace() {
		return deployContext.ClusterAPI.Client
	}
	return deployContext.ClusterAPI.NonCachedClient
}
