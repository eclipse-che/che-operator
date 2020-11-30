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
	blueprintObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", blueprint)
	}

	key := client.ObjectKey{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}

	actual := blueprintObject.DeepCopyObject()

	if getErr := deployContext.ClusterAPI.Client.Get(context.TODO(), key, actual); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return false, getErr
		}
		actual = nil
	}

	if actual == nil {
		_, err := create(deployContext, key, blueprint)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return update(deployContext, actual, blueprint, diffOpts)
}

func create(deployContext *DeployContext, key client.ObjectKey, blueprint metav1.Object) (runtime.Object, error) {
	blueprintObject, ok := blueprint.(runtime.Object)
	kind := blueprintObject.GetObjectKind().GroupVersionKind().Kind
	if !ok {
		return nil, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", blueprint)
	}

	actual := blueprintObject.DeepCopyObject()

	clusterAPI := deployContext.ClusterAPI
	logrus.Infof("Creating a new object: %s, name %s", kind, blueprint.GetName())
	obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
	if err != nil {
		return nil, err
	}

	err = clusterAPI.Client.Create(context.TODO(), obj)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return nil, err
		}

		// ok, we got an already-exists error. So let's try to load the object into "actual".
		// if we fail this retry for whatever reason, just give up rather than retrying this in a loop...
		// the reconciliation loop will lead us here again in the next round.
		if getErr := deployContext.ClusterAPI.Client.Get(context.TODO(), key, actual); getErr != nil {
			return nil, getErr
		}
	}

	return actual, nil
}

func update(deployContext *DeployContext, actual runtime.Object, blueprint metav1.Object, diffOpts cmp.Option) (bool, error) {
	clusterAPI := deployContext.ClusterAPI

	actualMeta := actual.(metav1.Object)

	diff := cmp.Diff(actual, blueprint, diffOpts)
	if len(diff) > 0 {
		kind := actual.GetObjectKind().GroupVersionKind().Kind
		logrus.Infof("Updating existing object: %s, name: %s", kind, actualMeta.GetName())
		fmt.Printf("Difference:\n%s", diff)

		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			err := clusterAPI.Client.Delete(context.TODO(), actual)
			if err != nil {
				return false, err
			}

			obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			err = clusterAPI.Client.Create(context.TODO(), obj)
			if err != nil {
				return false, err
			}
		} else {
			obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			obj.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())

			err = clusterAPI.Client.Update(context.TODO(), obj)
			if err != nil {
				return false, err
			}
		}

		return true, nil
	}
	return false, nil
}

func isUpdateUsingDeleteCreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind
}

func shouldSetOwnerReference(kind string) bool {
	return "OAuthClient" != kind
}

func setOwnerReferenceAndConvertToRuntime(deployContext *DeployContext, obj metav1.Object) (runtime.Object, error) {
	robj, ok := obj.(runtime.Object)
	if !ok {
		return nil, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", obj)
	}

	if !shouldSetOwnerReference(robj.GetObjectKind().GroupVersionKind().Kind) {
		return robj, nil
	}

	err := controllerutil.SetControllerReference(deployContext.CheCluster, obj, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return robj, nil
}
