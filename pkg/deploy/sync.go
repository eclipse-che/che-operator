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

// Sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner. The sync is always done using
// a delete and re-creation.
func Sync(deployContext *DeployContext, blueprint metav1.Object, diffOpts cmp.Option) error {
	clusterAPI := deployContext.ClusterAPI

	blueprintObject, ok := blueprint.(runtime.Object)
	if !ok {
		return fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", blueprint)
	}

	key := client.ObjectKey{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}

	actual := blueprintObject.DeepCopyObject()

	if getErr := deployContext.ClusterAPI.Client.Get(context.TODO(), key, actual); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return getErr
		}
		actual = nil
	}

	kind := blueprintObject.GetObjectKind().GroupVersionKind().Kind

	if actual == nil {
		logrus.Infof("Creating a new object: %s, name %s", kind, blueprint.GetName())
		obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
		if err != nil {
			return err
		}

		err = clusterAPI.Client.Create(context.TODO(), obj)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}

			// ok, we got an already-exists error. So let's try to load the object into "actual".
			// if we fail this retry for whatever reason, just give up rather than retrying this in a loop...
			// the reconciliation loop will lead us here again in the next round.
			if getErr := deployContext.ClusterAPI.Client.Get(context.TODO(), key, actual); getErr != nil {
				return getErr
			}
		}
	}

	if actual != nil {
		actualMeta := actual.(metav1.Object)

		diff := cmp.Diff(actual, blueprint, diffOpts)
		if len(diff) > 0 {
			logrus.Infof("Updating existing object: %s, name: %s", kind, actualMeta.GetName())
			fmt.Printf("Difference:\n%s", diff)

			if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
				err := clusterAPI.Client.Delete(context.TODO(), actual)
				if err != nil {
					return err
				}

				obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
				if err != nil {
					return err
				}

				err = clusterAPI.Client.Create(context.TODO(), obj)
				if err != nil {
					return err
				}
			} else {
				obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
				if err != nil {
					return err
				}

				// to be able to update, we need to set the resource version of the object that we know of
				obj.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())

				err = clusterAPI.Client.Update(context.TODO(), obj)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
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
