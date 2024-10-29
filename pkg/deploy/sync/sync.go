//
// Copyright (c) 2019-2024 Red Hat, Inc.
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

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	syncLog = ctrl.Log.WithName("sync")
)

type Syncer interface {
	// Get reads object.
	// Returns true if object exists otherwise returns false.
	// Returns error if object cannot be retrieved otherwise returns nil.
	Get(key client.ObjectKey, actual client.Object) (bool, error)
	// CreateIgnoreIfExists creates object.
	// Set owner reference for Eclipse Che namespace objects.
	// Return true if a new object is created or object already exists, otherwise returns false.
	// Returns error if object cannot be created otherwise returns nil.
	CreateIgnoreIfExists(blueprint client.Object) (bool, error)
	// Delete deletes object.
	// Returns true if object deleted or not found otherwise returns false.
	// Returns error if object cannot be deleted otherwise returns nil.
	Delete(key client.ObjectKey, objectMeta client.Object) (bool, error)
	// Sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner.
	// Returns true if object is up-to-date otherwise returns false
	// Returns error if object cannot be created/updated otherwise returns nil.
	Sync(blueprint client.Object, diffOpts ...cmp.Option) (bool, error)
}

type ObjSyncer struct {
	Syncer

	cheCluster *chev2.CheCluster
	scheme     *runtime.Scheme
	cli        client.Client
	ctx        context.Context
}

func (s *ObjSyncer) Get(key client.ObjectKey, actual client.Object) (bool, error) {
	return s.doGet(key, actual)
}

func (s *ObjSyncer) CreateIgnoreIfExists(blueprint client.Object) (bool, error) {
	return s.doCreate(blueprint, true)
}

func (s *ObjSyncer) Delete(key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return s.doDelete(key, objectMeta)
}

func (s *ObjSyncer) Sync(blueprint client.Object, diffOpts ...cmp.Option) (bool, error) {
	runtimeObject, ok := blueprint.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", runtimeObject)
	}

	// we will compare this object later with blueprint
	// we can't use runtimeObject.DeepCopyObject()
	actual, err := s.scheme.New(runtimeObject.GetObjectKind().GroupVersionKind())
	if err != nil {
		return false, err
	}

	key := types.NamespacedName{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}
	exists, err := s.doGet(key, actual.(client.Object))
	if err != nil {
		return false, err
	}

	// set GroupVersionKind (it might be empty)
	actual.GetObjectKind().SetGroupVersionKind(runtimeObject.GetObjectKind().GroupVersionKind())
	if !exists {
		return s.doCreate(blueprint, false)
	}

	return s.doUpdate(actual.(client.Object), blueprint, diffOpts...)
}

func (s *ObjSyncer) doUpdate(
	actual client.Object,
	blueprint client.Object,
	diffOpts ...cmp.Option,
) (bool, error) {
	actualMeta, ok := actual.(metav1.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a metav1.Object. Cannot update it", actualMeta)
	}

	diff := cmp.Diff(actual, blueprint, diffOpts...)
	if len(diff) > 0 {
		// don't print difference if there are no diffOpts mainly to avoid huge output
		if len(diffOpts) != 0 {
			fmt.Printf("Difference:\n%s", diff)
		}

		if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			done, err := s.doDeleteIgnoreIfNotFound(actual)
			if !done {
				return false, err
			}
			return s.doCreate(blueprint, false)
		} else {
			err := s.setOwnerReferenceForCheNamespaceObject(blueprint)
			if err != nil {
				return false, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			blueprint.(metav1.Object).SetResourceVersion(actualMeta.GetResourceVersion())
			err = s.cli.Update(context.TODO(), blueprint)
			if err == nil {
				syncLog.Info("Object updated", "namespace", actual.GetNamespace(), "kind", GetObjectType(actual), "name", actual.GetName())
			}
			return false, err
		}
	}

	return true, nil
}

func (s *ObjSyncer) doDelete(key client.ObjectKey, objectMeta client.Object) (bool, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object. Cannot delete it", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject().(client.Object)
	exists, err := s.doGet(key, actual)
	if !exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return s.doDeleteIgnoreIfNotFound(actual)
}

func (s *ObjSyncer) doDeleteIgnoreIfNotFound(actual client.Object) (bool, error) {
	err := s.cli.Delete(s.ctx, actual)
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

func (s *ObjSyncer) doGet(
	key client.ObjectKey,
	object client.Object,
) (bool, error) {
	err := s.cli.Get(s.ctx, key, object)
	if err == nil {
		return true, nil
	} else if errors.IsNotFound(err) {
		return false, nil
	} else {
		return false, err
	}
}

func (s *ObjSyncer) doCreate(
	blueprint client.Object,
	returnTrueIfAlreadyExists bool,
) (bool, error) {
	err := s.setOwnerReferenceForCheNamespaceObject(blueprint)
	if err != nil {
		return false, err
	}

	err = s.cli.Create(s.ctx, blueprint)
	if err == nil {
		syncLog.Info("Object created", "namespace", blueprint.GetNamespace(), "kind", GetObjectType(blueprint), "name", blueprint.GetName())
		return true, nil
	} else if errors.IsAlreadyExists(err) {
		return returnTrueIfAlreadyExists, nil
	} else {
		return false, err
	}
}

func (s *ObjSyncer) setOwnerReferenceForCheNamespaceObject(blueprint metav1.Object) error {
	if blueprint.GetNamespace() == s.cheCluster.Namespace {
		return controllerutil.SetControllerReference(s.cheCluster, blueprint, s.scheme)
	}

	// cluster scope object (empty workspace) or object in another namespace
	return nil
}

func GetObjectType(obj interface{}) string {
	objType := reflect.TypeOf(obj).String()
	if reflect.TypeOf(obj).Kind().String() == "ptr" {
		objType = objType[1:]
	}

	return objType
}

func isUpdateUsingDeleteCreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind || "Job" == kind || "Secret" == kind
}
