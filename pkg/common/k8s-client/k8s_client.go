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

package k8s_client

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger = ctrl.Log.WithName("sync")
)

func NewK8sClient(cli client.Client, scheme *runtime.Scheme) *K8sClient {
	return &K8sClient{cli: cli, scheme: scheme}
}

type K8sClient struct {
	cli    client.Client
	scheme *runtime.Scheme
}

func (s K8sClient) Sync(
	blueprint client.Object,
	owner metav1.Object,
	diffOpts ...cmp.Option,
) (bool, error) {
	// we will compare this object later with blueprint
	actual, err := s.scheme.New(blueprint.GetObjectKind().GroupVersionKind())
	if err != nil {
		return false, err
	}

	key := types.NamespacedName{
		Name:      blueprint.GetName(),
		Namespace: blueprint.GetNamespace(),
	}
	if exists, err := s.doGetIgnoreNotFound(key, actual.(client.Object)); err != nil {
		return false, err
	} else if !exists {
		return s.doSync(nil, blueprint, owner, diffOpts...)
	}

	return s.doSync(actual.(client.Object), blueprint, owner, diffOpts...)
}

func (s K8sClient) Create(blueprint client.Object, owner metav1.Object) (bool, error) {
	return s.doCreate(blueprint, owner, false)
}

func (s K8sClient) CreateIgnoreIfExists(blueprint client.Object, owner metav1.Object) (bool, error) {
	return s.doCreate(blueprint, owner, true)
}

func (s K8sClient) Get(key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return s.doGetIgnoreNotFound(key, objectMeta)
}

func (s K8sClient) GetClusterScoped(name string, objectMeta client.Object) (bool, error) {
	return s.doGetIgnoreNotFound(types.NamespacedName{Name: name}, objectMeta)
}

func (s K8sClient) Delete(key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return s.deleteByKeyIgnoreNotFound(key, objectMeta)
}

func (s K8sClient) DeleteClusterClusterScoped(name string, objectMeta client.Object) (bool, error) {
	return s.deleteByKeyIgnoreNotFound(types.NamespacedName{Name: name}, objectMeta)
}

func (s K8sClient) List(list client.ObjectList, opts ...client.ListOption) ([]runtime.Object, error) {
	err := s.cli.List(context.TODO(), list, opts...)
	if err != nil {
		return []runtime.Object{}, err
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return []runtime.Object{}, err
	}

	for i, _ := range items {
		if gvk, err := apiutil.GVKForObject(items[i], s.scheme); err != nil {
			return nil, err
		} else {
			items[i].GetObjectKind().SetGroupVersionKind(gvk)
		}
	}

	return items, nil
}

// deleteByKeyIgnoreNotFound deletes object by key.
// Returns true if object deleted or not found otherwise returns false.
// Returns error if object cannot be deleted otherwise returns nil.
func (s K8sClient) deleteByKeyIgnoreNotFound(key client.ObjectKey, objectMeta client.Object) (bool, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject().(client.Object)
	if exists, err := s.doGetIgnoreNotFound(key, actual); !exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return s.doDeleteIgnoreIfNotFound(actual)
}

// doDeleteIgnoreIfNotFound deletes object.
// Returns true if object deleted or not found otherwise returns false.
// Returns error if object cannot be deleted otherwise returns nil.
func (s K8sClient) doDeleteIgnoreIfNotFound(object client.Object) (bool, error) {
	if err := s.cli.Delete(context.TODO(), object); err == nil {
		if errors.IsNotFound(err) {
			logger.Info("Object not found", "namespace", object.GetNamespace(), "kind", GetObjectType(object), "name", object.GetName())
		} else {
			logger.Info("Object deleted", "namespace", object.GetNamespace(), "kind", GetObjectType(object), "name", object.GetName())
		}
		return true, nil
	} else {
		return false, err
	}
}

// doGet gets object.
// Returns true if object exists otherwise returns false.
// Returns error if object cannot be retrieved otherwise returns nil.
func (s K8sClient) doGetIgnoreNotFound(key client.ObjectKey, object client.Object) (bool, error) {
	if err := s.cli.Get(context.TODO(), key, object); err == nil {
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
func (s K8sClient) doCreate(blueprint client.Object, owner metav1.Object, ignoreIfAlreadyExists bool,
) (bool, error) {
	if err := s.setOwner(blueprint, owner); err != nil {
		return false, err
	}

	if err := s.cli.Create(context.TODO(), blueprint); err == nil {
		logger.Info("Object created", "namespace", blueprint.GetNamespace(), "kind", GetObjectType(blueprint), "name", blueprint.GetName())
		return true, nil
	} else if errors.IsAlreadyExists(err) {
		if ignoreIfAlreadyExists {
			logger.Info("Object already exists, ignoring", "namespace", blueprint.GetNamespace(), "kind", GetObjectType(blueprint), "name", blueprint.GetName())
			return true, nil
		} else {
			return false, err
		}
	} else {
		return false, err
	}
}

// doSync ensures that the object is up to date in the cluster.
// Return true if a new object is synced, otherwise returns false.
// Returns error if object cannot be synced otherwise returns nil.
func (s K8sClient) doSync(
	actual client.Object,
	blueprint client.Object,
	owner metav1.Object,
	diffOpts ...cmp.Option,
) (bool, error) {
	if actual == nil {
		return s.doCreate(blueprint, owner, false)
	}

	// set GroupVersionKind (it might be empty)
	actual.GetObjectKind().SetGroupVersionKind(blueprint.GetObjectKind().GroupVersionKind())

	diff := cmp.Diff(actual, blueprint, diffOpts...)
	if len(diff) > 0 {
		// don't print difference if there are no diffOpts mainly to avoid huge output
		if len(diffOpts) != 0 {
			fmt.Printf("Difference:\n%s", diff)
		}

		if s.isRecreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			if done, err := s.doDeleteIgnoreIfNotFound(actual); !done {
				return false, err
			}
			return s.doCreate(blueprint, owner, false)
		} else {
			if err := s.setOwner(blueprint, owner); err != nil {
				return false, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			blueprint.(metav1.Object).SetResourceVersion(actual.GetResourceVersion())

			err := s.cli.Update(context.TODO(), blueprint)
			if err == nil {
				logger.Info("Object updated", "namespace", actual.GetNamespace(), "kind", GetObjectType(actual), "name", actual.GetName())
			}
			return false, err
		}
	}

	return true, nil
}

// setOwner sets owner to the object
func (s K8sClient) setOwner(blueprint client.Object, owner metav1.Object) error {
	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, blueprint, s.scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	return nil
}

// isRecreate returns true, if object should be deleted/created instead of being updated.
func (s K8sClient) isRecreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind || "Job" == kind || "Secret" == kind
}

func GetObjectType(obj interface{}) string {
	objType := reflect.TypeOf(obj).String()
	if reflect.TypeOf(obj).Kind().String() == "ptr" {
		objType = objType[1:]
	}

	return objType
}
