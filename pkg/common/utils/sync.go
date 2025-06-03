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

package utils

import (
	"context"
	"fmt"
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	syncLog = ctrl.Log.WithName("sync")
)

type Syncer interface {
	// Get gets object.
	// Returns true if object exists otherwise returns false.
	// Returns error if object cannot be retrieved otherwise returns nil.
	Get(key client.ObjectKey, objectMeta client.Object) (bool, error)
	// Delete does delete object by key.
	// Returns true if object deleted or not found otherwise returns false.
	// Returns error if object cannot be deleted otherwise returns nil.
	Delete(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error)
}

type ObjSyncer struct {
	syncer Syncer
	cli    client.Client
}

func (s ObjSyncer) Get(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return s.doGetIgnoreNotFound(context, key, objectMeta)
}

func (s ObjSyncer) Delete(context context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return s.deleteByKeyIgnoreNotFound(context, key, objectMeta)
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

func GetObjectType(obj interface{}) string {
	objType := reflect.TypeOf(obj).String()
	if reflect.TypeOf(obj).Kind().String() == "ptr" {
		objType = objType[1:]
	}

	return objType
}
