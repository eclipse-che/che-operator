//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
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
	exists, err := GetWithClient(cli, key, actual.(client.Object))
	if err != nil {
		return false, err
	}

	// set GroupVersionKind (it might be empty)
	actual.GetObjectKind().SetGroupVersionKind(runtimeObject.GetObjectKind().GroupVersionKind())
	if !exists {
		return CreateWithClient(cli, deployContext, blueprint, false)
	}

	return UpdateWithClient(cli, deployContext, actual.(client.Object), blueprint, diffOpts...)
}

func SyncAndAddFinalizer(
	deployContext *chetypes.DeployContext,
	blueprint metav1.Object,
	diffOpts cmp.Option,
	finalizer string) (bool, error) {

	// eclipse-che custom resource is being deleted, we shouldn't sync
	// TODO move this check before `Sync` invocation
	if deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		done, err := Sync(deployContext, blueprint.(client.Object), diffOpts)
		if !done {
			return done, err
		}
		err = AppendFinalizer(deployContext, finalizer)
		return err == nil, err
	}
	return true, nil
}

// Gets object by key.
// Returns true if object exists otherwise returns false.
func Get(deployContext *chetypes.DeployContext, key client.ObjectKey, actual client.Object) (bool, error) {
	cli := getClientForObject(key.Namespace, deployContext)
	return GetWithClient(cli, key, actual)
}

// Gets namespaced scope object by name
// Returns true if object exists otherwise returns false.
func GetNamespacedObject(deployContext *chetypes.DeployContext, name string, actual client.Object) (bool, error) {
	client := deployContext.ClusterAPI.Client
	key := types.NamespacedName{Name: name, Namespace: deployContext.CheCluster.Namespace}
	return GetWithClient(client, key, actual)
}

// Gets cluster scope object by name
// Returns true if object exists otherwise returns false
func GetClusterObject(deployContext *chetypes.DeployContext, name string, actual client.Object) (bool, error) {
	client := deployContext.ClusterAPI.NonCachingClient
	key := types.NamespacedName{Name: name}
	return GetWithClient(client, key, actual)
}

// Creates object.
// Return true if a new object is created, false if it has been already created or error occurred.
func CreateIfNotExists(deployContext *chetypes.DeployContext, blueprint client.Object) (isCreated bool, err error) {
	cli := getClientForObject(blueprint.GetNamespace(), deployContext)
	return CreateIfNotExistsWithClient(cli, deployContext, blueprint)
}

func CreateIfNotExistsWithClient(cli client.Client, deployContext *chetypes.DeployContext, blueprint client.Object) (isCreated bool, err error) {
	key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}
	actual := blueprint.DeepCopyObject().(client.Object)
	exists, err := GetWithClient(cli, key, actual)
	if exists {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return CreateWithClient(cli, deployContext, blueprint, false)
}

// Creates object.
// Return true if a new object is created otherwise returns false.
func Create(deployContext *chetypes.DeployContext, blueprint client.Object) (bool, error) {
	client := getClientForObject(blueprint.GetNamespace(), deployContext)
	return CreateWithClient(client, deployContext, blueprint, false)
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

// Updates object.
// Returns true if object is up to date otherwiser return false
func UpdateWithClient(client client.Client, deployContext *chetypes.DeployContext, actual client.Object, blueprint client.Object, diffOpts ...cmp.Option) (bool, error) {
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
			done, err := DeleteWithClient(client, actual)
			if !done {
				return false, err
			}
			return CreateWithClient(client, deployContext, blueprint, false)
		} else {
			logrus.Infof("Updating existing object: %s, name: %s", GetObjectType(actualMeta), actualMeta.GetName())
			err := setOwnerReferenceIfNeeded(deployContext, blueprint)
			if err != nil {
				return false, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			blueprint.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())
			err = client.Update(context.TODO(), blueprint)
			return false, err
		}
	}
	return true, nil
}

func CreateWithClient(client client.Client, deployContext *chetypes.DeployContext, blueprint client.Object, returnTrueIfAlreadyExists bool) (bool, error) {
	logrus.Infof("Creating a new object: %s, name: %s", GetObjectType(blueprint), blueprint.GetName())

	err := setOwnerReferenceIfNeeded(deployContext, blueprint)
	if err != nil {
		return false, err
	}

	err = client.Create(context.TODO(), blueprint)
	if err == nil {
		return true, nil
	} else if errors.IsAlreadyExists(err) {
		return returnTrueIfAlreadyExists, nil
	} else {
		return false, err
	}
}

func DeleteByKeyWithClient(cli client.Client, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject().(client.Object)
	exists, err := GetWithClient(cli, key, actual)
	if !exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return DeleteWithClient(cli, actual)
}

func DeleteWithClient(client client.Client, actual client.Object) (bool, error) {
	logrus.Infof("Deleting object: %s, name: %s", GetObjectType(actual), actual.GetName())

	err := client.Delete(context.TODO(), actual)
	if err == nil || errors.IsNotFound(err) {
		return true, nil
	} else {
		return false, err
	}
}

func GetWithClient(client client.Client, key client.ObjectKey, object client.Object) (bool, error) {
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
