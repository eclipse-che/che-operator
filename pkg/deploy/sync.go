//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package deploy

import (
	"context"
	"fmt"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	syncLog = ctrl.Log.WithName("sync")
)

// Sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner.
// Returns true if object is up-to-date otherwise returns false
func Sync(deployContext *chetypes.DeployContext, blueprint client.Object, diffOpts ...cmp.Option) (bool, error) {
	cli := getClientForObject(blueprint.GetNamespace(), deployContext)
	return SyncWithClient(cli, deployContext, blueprint, diffOpts...)
}

func SyncWithClient(cli client.Client, deployContext *chetypes.DeployContext, blueprint client.Object, diffOpts ...cmp.Option) (bool, error) {
	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	// we will compare this object later with blueprint
	// we can't use runtimeObject.DeepCopyObject()
	actual, err := deployContext.ClusterAPI.Scheme.New(runtimeObject.GetObjectKind().GroupVersionKind())
	if err != nil {
		return false, err
	}

	key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}
	exists, err := doGet(context.TODO(), cli, key, actual.(client.Object))
	if err != nil {
		return false, err
	}

	// set GroupVersionKind (it might be empty)
	actual.GetObjectKind().SetGroupVersionKind(runtimeObject.GetObjectKind().GroupVersionKind())
	if !exists {
		return doCreate(context.TODO(), cli, deployContext, blueprint, false)
	}

	return doUpdate(cli, deployContext, actual.(client.Object), blueprint, diffOpts...)
}

// Get gets object.
// Returns true if object exists otherwise returns false.
// Returns error if object cannot be retrieved otherwise returns nil.
func Get(deployContext *chetypes.DeployContext, key client.ObjectKey, actual client.Object) (bool, error) {
	cli := getClientForObject(key.Namespace, deployContext)
	return doGet(context.TODO(), cli, key, actual)
}

// Gets namespaced scope object by name
// Returns true if object exists otherwise returns false.
func GetNamespacedObject(deployContext *chetypes.DeployContext, name string, actual client.Object) (bool, error) {
	client := deployContext.ClusterAPI.Client
	key := types.NamespacedName{Name: name, Namespace: deployContext.CheCluster.Namespace}
	return doGet(context.TODO(), client, key, actual)
}

// Gets cluster scope object by name
// Returns true if object exists otherwise returns false
func GetClusterObject(deployContext *chetypes.DeployContext, name string, actual client.Object) (bool, error) {
	client := deployContext.ClusterAPI.NonCachingClient
	key := types.NamespacedName{Name: name}
	return doGet(context.TODO(), client, key, actual)
}

// CreateIgnoreIfExists creates object.
// Return true if a new object is created or object already exists, otherwise returns false.
// Throws error if object cannot be created otherwise returns nil.
func CreateIgnoreIfExists(deployContext *chetypes.DeployContext, blueprint client.Object) (bool, error) {
	cli := getClientForObject(blueprint.GetNamespace(), deployContext)
	return doCreate(context.TODO(), cli, deployContext, blueprint, true)
}

// Deletes object.
// Returns true if object deleted or not found otherwise returns false.
func Delete(deployContext *chetypes.DeployContext, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	client := getClientForObject(key.Namespace, deployContext)
	return DeleteByKeyWithClient(client, key, objectMeta)
}

func DeleteNamespacedObject(deployContext *chetypes.DeployContext, name string, objectMeta client.Object) (bool, error) {
	client := deployContext.ClusterAPI.Client
	key := types.NamespacedName{Name: name, Namespace: deployContext.CheCluster.Namespace}
	return DeleteByKeyWithClient(client, key, objectMeta)
}

func DeleteClusterObject(deployContext *chetypes.DeployContext, name string, objectMeta client.Object) (bool, error) {
	client := deployContext.ClusterAPI.NonCachingClient
	key := types.NamespacedName{Name: name}
	return DeleteByKeyWithClient(client, key, objectMeta)
}

func DeleteByKeyWithClient(cli client.Client, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject().(client.Object)
	exists, err := doGet(context.TODO(), cli, key, actual)
	if !exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return doDeleteIgnoreIfNotFound(context.TODO(), cli, actual)
}

func isUpdateUsingDeleteCreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind || "Job" == kind || "Secret" == kind
}

func setOwnerReferenceIfNeeded(deployContext *chetypes.DeployContext, blueprint metav1.Object) error {
	if shouldSetOwnerReferenceForObject(deployContext, blueprint) {
		return controllerutil.SetControllerReference(deployContext.CheCluster, blueprint, deployContext.ClusterAPI.Scheme)
	}

	return nil
}

func shouldSetOwnerReferenceForObject(deployContext *chetypes.DeployContext, blueprint metav1.Object) bool {
	// empty workspace (cluster scope object) or object in another namespace
	return blueprint.GetNamespace() == deployContext.CheCluster.Namespace
}

func getClientForObject(objectNamespace string, deployContext *chetypes.DeployContext) client.Client {
	// empty namespace (cluster scope object) or object in another namespace
	if deployContext.CheCluster.Namespace == objectNamespace {
		return deployContext.ClusterAPI.Client
	}
	return deployContext.ClusterAPI.NonCachingClient
}

func GetObjectType(obj interface{}) string {
	objType := reflect.TypeOf(obj).String()
	if reflect.TypeOf(obj).Kind().String() == "ptr" {
		objType = objType[1:]
	}

	return objType
}

// DeleteIgnoreIfNotFound deletes object.
// Returns nil if object deleted or not found otherwise returns error.
// Return error if object cannot be deleted otherwise returns nil.
func DeleteIgnoreIfNotFound(
	context context.Context,
	cli client.Client,
	key client.ObjectKey,
	blueprint client.Object,
) error {
	runtimeObj, ok := blueprint.(runtime.Object)
	if !ok {
		return fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObj)
	}

	actual := runtimeObj.DeepCopyObject().(client.Object)

	exists, err := doGet(context, cli, key, actual)
	if exists {
		_, err := doDeleteIgnoreIfNotFound(context, cli, actual)
		return err
	}

	return err
}

// doCreate creates object.
// Return error if object cannot be created otherwise returns nil.
func doCreate(
	context context.Context,
	client client.Client,
	deployContext *chetypes.DeployContext,
	blueprint client.Object,
	returnTrueIfAlreadyExists bool,
) (bool, error) {
	err := setOwnerReferenceIfNeeded(deployContext, blueprint)
	if err != nil {
		return false, err
	}

	err = client.Create(context, blueprint)
	if err == nil {
		syncLog.Info("Object created", "namespace", blueprint.GetNamespace(), "kind", GetObjectType(blueprint), "name", blueprint.GetName())
		return true, nil
	} else if errors.IsAlreadyExists(err) {
		return returnTrueIfAlreadyExists, nil
	} else {
		return false, err
	}
}

// doDeleteIgnoreIfNotFound deletes object.
// Returns true if object deleted or not found otherwise returns false.
// Returns error if object cannot be deleted otherwise returns nil.
func doDeleteIgnoreIfNotFound(
	context context.Context,
	cli client.Client,
	actual client.Object,
) (bool, error) {
	err := cli.Delete(context, actual)
	if err == nil {
		if errors.IsNotFound(err) {
			syncLog.Info("Object not found", "namespace", actual.GetNamespace(), "kind", GetObjectType(actual), "name", actual.GetName())
		} else {
			syncLog.Info("Object deleted", "namespace", actual.GetNamespace(), "kind", GetObjectType(actual), "name", actual.GetName())
		}
		return true, nil
	} else {
		return false, err
	}
}

// doGet gets object.
// Returns true if object exists otherwise returns false.
// Returns error if object cannot be retrieved otherwise returns nil.
func doGet(
	context context.Context,
	cli client.Client,
	key client.ObjectKey,
	object client.Object,
) (bool, error) {
	err := cli.Get(context, key, object)
	if err == nil {
		return true, nil
	} else if errors.IsNotFound(err) {
		return false, nil
	} else {
		return false, err
	}
}

// doUpdate updates object.
// Returns true if object is up-to-date otherwise return false
func doUpdate(
	cli client.Client,
	deployContext *chetypes.DeployContext,
	actual client.Object,
	blueprint client.Object,
	diffOpts ...cmp.Option,
) (bool, error) {
	actualMeta, ok := actual.(metav1.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a metav1.Object. Cannot sync it", actualMeta)
	}

	diff := cmp.Diff(actual, blueprint, diffOpts...)
	if len(diff) > 0 {
		// don't print difference if there are no diffOpts mainly to avoid huge output
		if len(diffOpts) != 0 {
			fmt.Printf("Difference:\n%s", diff)
		}

		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			done, err := doDeleteIgnoreIfNotFound(context.TODO(), cli, actual)
			if !done {
				return false, err
			}
			return doCreate(context.TODO(), cli, deployContext, blueprint, false)
		} else {
			err := setOwnerReferenceIfNeeded(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			blueprint.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())
			err = cli.Update(context.TODO(), blueprint)
			if err == nil {
				syncLog.Info("Object updated", "namespace", actual.GetNamespace(), "kind", GetObjectType(actual), "name", actual.GetName())
			}
			return false, err
		}
	}

	return true, nil
}
