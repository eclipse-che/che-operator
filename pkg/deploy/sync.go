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
package deploy

import (
	"context"
	"fmt"
	"reflect"

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
func Sync(deployContext *DeployContext, blueprint metav1.Object, diffOpts ...cmp.Option) (bool, error) {
	// eclipse-che custom resource is being deleted, we shouldn't sync
	// TODO move this check before `Sync` invocation
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return true, nil
	}

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

	client := getClientForObject(blueprint.GetNamespace(), deployContext)
	key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}

	exists, err := doGet(client, key, actual)
	if err != nil {
		return false, err
	}

	if !exists {
		return Create(deployContext, blueprint)
	}
	return Update(deployContext, actual, blueprint, diffOpts...)
}

func SyncAndAddFinalizer(
	deployContext *DeployContext,
	blueprint metav1.Object,
	diffOpts cmp.Option,
	finalizer string) (bool, error) {

	// eclipse-che custom resource is being deleted, we shouldn't sync
	// TODO move this check before `Sync` invocation
	if deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		done, err := Sync(deployContext, blueprint, diffOpts)
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
func Get(deployContext *DeployContext, key client.ObjectKey, actual metav1.Object) (bool, error) {
	runtimeObject, ok := actual.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	client := getClientForObject(key.Namespace, deployContext)
	return doGet(client, key, runtimeObject)
}

// Gets namespaced scope object by name
// Returns true if object exists otherwise returns false.
func GetNamespacedObject(deployContext *DeployContext, name string, actual metav1.Object) (bool, error) {
	runtimeObject, ok := actual.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	client := deployContext.ClusterAPI.Client
	key := types.NamespacedName{Name: name, Namespace: deployContext.CheCluster.Namespace}
	return doGet(client, key, runtimeObject)
}

// Gets cluster scope object by name
// Returns true if object exists otherwise returns false
func GetClusterObject(deployContext *DeployContext, name string, actual metav1.Object) (bool, error) {
	runtimeObject, ok := actual.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	client := deployContext.ClusterAPI.NonCachedClient
	key := types.NamespacedName{Name: name}
	return doGet(client, key, runtimeObject)
}

// Creates object.
// Return true if a new object is created or has been already created otherwise returns false.
func CreateIfNotExists(deployContext *DeployContext, blueprint metav1.Object) (bool, error) {
	// eclipse-che custom resource is being deleted, we shouldn't sync
	// TODO move this check before `Sync` invocation
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return true, nil
	}

	client := getClientForObject(blueprint.GetNamespace(), deployContext)
	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}
	actual := runtimeObject.DeepCopyObject()
	exists, err := doGet(client, key, actual)
	if exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	logrus.Infof("Creating a new object: %s, name: %s", getObjectType(blueprint), blueprint.GetName())

	err = setOwnerReferenceIfNeeded(deployContext, blueprint)
	if err != nil {
		return false, err
	}

	return doCreate(client, runtimeObject, true)
}

// Creates object.
// Return true if a new object is created otherwise returns false.
func Create(deployContext *DeployContext, blueprint metav1.Object) (bool, error) {
	// eclipse-che custom resource is being deleted, we shouldn't sync
	// TODO move this check before `Sync` invocation
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return true, nil
	}

	client := getClientForObject(blueprint.GetNamespace(), deployContext)
	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	logrus.Infof("Creating a new object: %s, name: %s", getObjectType(blueprint), blueprint.GetName())

	err := setOwnerReferenceIfNeeded(deployContext, blueprint)
	if err != nil {
		return false, err
	}

	return doCreate(client, runtimeObject, false)
}

// Deletes object.
// Returns true if object deleted or not found otherwise returns false.
func Delete(deployContext *DeployContext, key client.ObjectKey, objectMeta metav1.Object) (bool, error) {
	client := getClientForObject(key.Namespace, deployContext)
	return doDeleteByKey(client, deployContext.ClusterAPI.Scheme, key, objectMeta)
}

func DeleteNamespacedObject(deployContext *DeployContext, name string, objectMeta metav1.Object) (bool, error) {
	client := deployContext.ClusterAPI.Client
	key := types.NamespacedName{Name: name, Namespace: deployContext.CheCluster.Namespace}
	return doDeleteByKey(client, deployContext.ClusterAPI.Scheme, key, objectMeta)
}

func DeleteClusterObject(deployContext *DeployContext, name string, objectMeta metav1.Object) (bool, error) {
	client := deployContext.ClusterAPI.NonCachedClient
	key := types.NamespacedName{Name: name}
	return doDeleteByKey(client, deployContext.ClusterAPI.Scheme, key, objectMeta)
}

// Updates object.
// Returns true if object is up to date otherwiser return false
func Update(deployContext *DeployContext, actual runtime.Object, blueprint metav1.Object, diffOpts ...cmp.Option) (bool, error) {
	// eclipse-che custom resource is being deleted, we shouldn't sync
	// TODO move this check before `Sync` invocation
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return true, nil
	}

	actualMeta, ok := actual.(metav1.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a metav1.Object. Cannot sync it", actualMeta)
	}

	diff := cmp.Diff(actual, blueprint, diffOpts...)
	if len(diff) > 0 {
		logrus.Infof("Updating existing object: %s, name: %s", getObjectType(actualMeta), actualMeta.GetName())

		// don't print difference if there are no diffOpts mainly to avoid huge output
		if len(diffOpts) != 0 {
			fmt.Printf("Difference:\n%s", diff)
		}

		targetLabels := map[string]string{}
		targetAnnos := map[string]string{}

		for k, v := range actualMeta.GetAnnotations() {
			targetAnnos[k] = v
		}
		for k, v := range actualMeta.GetLabels() {
			targetLabels[k] = v
		}

		for k, v := range blueprint.GetAnnotations() {
			targetAnnos[k] = v
		}
		for k, v := range blueprint.GetLabels() {
			targetLabels[k] = v
		}

		blueprint.SetAnnotations(targetAnnos)
		blueprint.SetLabels(targetLabels)

		client := getClientForObject(actualMeta.GetNamespace(), deployContext)
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

func doDeleteByKey(client client.Client, scheme *runtime.Scheme, key client.ObjectKey, objectMeta metav1.Object) (bool, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject()
	exists, err := doGet(client, key, actual)
	if !exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	logrus.Infof("Deleting object: %s, name: %s", getObjectType(objectMeta), key.Name)

	return doDelete(client, actual)
}

func doDelete(client client.Client, actual runtime.Object) (bool, error) {
	err := client.Delete(context.TODO(), actual)
	if err == nil || errors.IsNotFound(err) {
		return true, nil
	} else {
		return false, err
	}
}

func doUpdate(client client.Client, object runtime.Object) (bool, error) {
	err := client.Update(context.TODO(), object)
	return false, err
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
	return "Service" == kind || "Ingress" == kind || "Route" == kind || "Job" == kind || "Secret" == kind
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

func getClientForObject(objectNamespace string, deployContext *DeployContext) client.Client {
	// empty namespace (cluster scope object) or object in another namespace
	if deployContext.CheCluster.Namespace == objectNamespace {
		return deployContext.ClusterAPI.Client
	}
	return deployContext.ClusterAPI.NonCachedClient
}

func getObjectType(objectMeta metav1.Object) string {
	objType := reflect.TypeOf(objectMeta).String()
	if reflect.TypeOf(objectMeta).Kind().String() == "ptr" {
		objType = objType[1:]
	}

	return objType
}
