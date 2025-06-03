//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package sync

import (
	"context"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	syncLog = ctrl.Log.WithName("sync")
)

type Syncer interface {
	// Create creates object.
	// Return true if a new object is created, otherwise returns false.
	// Returns error if object cannot be created otherwise returns nil.
	Create(context context.Context, blueprint client.Object, owner metav1.Object) (bool, error)
	// CreateIgnoreIfExists creates object.
	// Return true if a new object is created or object already exists, otherwise returns false.
	// Returns error if object cannot be created otherwise returns nil.
	CreateIgnoreIfExists(context context.Context, blueprint client.Object, owner metav1.Object) (bool, error)
	// Get gets object.
	// Returns true if object exists otherwise returns false.
	// Returns error if object cannot be retrieved otherwise returns nil.
	Get(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error)
	// GetClusterScoped gets cluster scoped object by name
	// Returns true if object exists otherwise returns false.
	// Returns error if object cannot be retrieved otherwise returns nil.
	GetClusterScoped(context context.Context, name string, objectMeta client.Object) (bool, error)
	// Delete deletes object by key.
	// Returns true if object deleted or not found otherwise returns false.
	// Returns error if object cannot be deleted otherwise returns nil.
	Delete(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error)
	// DeleteClusterClusterScoped deletes cluster scoped object by name.
	// Returns true if object deleted or not found otherwise returns false.
	// Returns error if object cannot be deleted otherwise returns nil.
	DeleteClusterClusterScoped(context context.Context, name string, objectMeta client.Object) (bool, error)
}

type ObjSyncer struct {
	syncer Syncer
	cli    client.Client
	scheme *runtime.Scheme
}

func (s ObjSyncer) Create(context context.Context, blueprint client.Object, owner metav1.Object) (bool, error) {
	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, blueprint, s.scheme); err != nil {
			return false, fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	return s.doCreate(context, blueprint, false)
}

func (s ObjSyncer) CreateIgnoreIfExists(context context.Context, blueprint client.Object, owner metav1.Object) (bool, error) {
	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, blueprint, s.scheme); err != nil {
			return false, fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	return s.doCreate(context, blueprint, true)
}

func (s ObjSyncer) Get(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return s.doGetIgnoreNotFound(context, key, objectMeta)
}

func (s ObjSyncer) GetClusterScoped(context context.Context, name string, objectMeta client.Object) (bool, error) {
	return s.doGetIgnoreNotFound(context, types.NamespacedName{Name: name}, objectMeta)
}

func (s ObjSyncer) Delete(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return s.deleteByKeyIgnoreNotFound(context, key, objectMeta)
}

func (s ObjSyncer) DeleteClusterClusterScoped(context context.Context, name string, objectMeta client.Object) (bool, error) {
	return s.deleteByKeyIgnoreNotFound(context, types.NamespacedName{Name: name}, objectMeta)
}

// deleteByKeyIgnoreNotFound deletes object by key.
// Returns true if object deleted or not found otherwise returns false.
// Returns error if object cannot be deleted otherwise returns nil.
func (s ObjSyncer) deleteByKeyIgnoreNotFound(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject().(client.Object)
	if exists, err := s.doGetIgnoreNotFound(context, key, actual); !exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return s.doDeleteIgnoreIfNotFound(context, actual)
}

// doDeleteIgnoreIfNotFound deletes object.
// Returns true if object deleted or not found otherwise returns false.
// Returns error if object cannot be deleted otherwise returns nil.
func (s ObjSyncer) doDeleteIgnoreIfNotFound(
	context context.Context,
	object client.Object,
) (bool, error) {
	if err := s.cli.Delete(context, object); err == nil {
		if errors.IsNotFound(err) {
			syncLog.Info("Object not found", "namespace", object.GetNamespace(), "kind", GetObjectType(object), "name", object.GetName())
		} else {
			syncLog.Info("Object deleted", "namespace", object.GetNamespace(), "kind", GetObjectType(object), "name", object.GetName())
		}
		return true, nil
	} else {
		return false, err
	}
}

// doGet gets object.
// Returns true if object exists otherwise returns false.
// Returns error if object cannot be retrieved otherwise returns nil.
func (s ObjSyncer) doGetIgnoreNotFound(
	context context.Context,
	key client.ObjectKey,
	object client.Object,
) (bool, error) {
	if err := s.cli.Get(context, key, object); err == nil {
		return true, nil
	} else if errors.IsNotFound(err) {
		return false, nil
	} else {
		return false, err
	}
}

// doCreate creates object.
// Returns true if object created or already exists otherwise returns false.
// Return error if object cannot be created otherwise returns nil.
func (s ObjSyncer) doCreate(
	context context.Context,
	blueprint client.Object,
	ignoreIfAlreadyExists bool,
) (bool, error) {
	if err := s.cli.Create(context, blueprint); err == nil {
		syncLog.Info("Object created", "namespace", blueprint.GetNamespace(), "kind", GetObjectType(blueprint), "name", blueprint.GetName())
		return true, nil
	} else if errors.IsAlreadyExists(err) {
		if ignoreIfAlreadyExists {
			syncLog.Info("Object already exists, ignoring", "namespace", blueprint.GetNamespace(), "kind", GetObjectType(blueprint), "name", blueprint.GetName())
			return true, nil
		} else {
			return false, err
		}
	} else {
		return false, err
	}
}

func GetObjectType(obj interface{}) string {
	objType := reflect.TypeOf(obj).String()
	if reflect.TypeOf(obj).Kind().String() == "ptr" {
		objType = objType[1:]
	}

	return objType
}
