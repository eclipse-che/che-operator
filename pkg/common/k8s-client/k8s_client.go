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
	logger = ctrl.Log.WithName("k8s")
)

type K8sClient interface {
	// Sync ensures that the object is up to date in the cluster.
	// Return true if a new object is synced, otherwise returns false.
	// Returns error if object cannot be synced otherwise returns nil.
	Sync(ctx context.Context, blueprint client.Object, owner metav1.Object, diffOpts ...cmp.Option) (bool, error)
	// Create creates object.
	// Return true if a new object is created, otherwise returns false.
	// Returns error if object cannot be created otherwise returns nil.
	Create(ctx context.Context, blueprint client.Object, owner metav1.Object) (bool, error)
	// CreateIgnoreIfExists creates object.
	// Return true if a new object is created or object already exists, otherwise returns false.
	// Returns error if object cannot be created otherwise returns nil.
	CreateIgnoreIfExists(ctx context.Context, blueprint client.Object, owner metav1.Object) (bool, error)
	// Get gets object.
	// Returns true if object exists otherwise returns false.
	// Returns error if object cannot be retrieved otherwise returns nil.
	Get(ctx context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error)
	// GetClusterScoped gets cluster scoped object by name
	// Returns true if object exists otherwise returns false.
	// Returns error if object cannot be retrieved otherwise returns nil.
	GetClusterScoped(ctx context.Context, name string, objectMeta client.Object) (bool, error)
	// Delete deletes object by key.
	// Returns true if object deleted or not found otherwise returns false.
	// Returns error if object cannot be deleted otherwise returns nil.
	Delete(ctx context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error)
	// DeleteClusterScoped deletes cluster scoped object by name.
	// Returns true if object deleted or not found otherwise returns false.
	// Returns error if object cannot be deleted otherwise returns nil.
	DeleteClusterScoped(ctx context.Context, name string, objectMeta client.Object) (bool, error)
	// List lists objects and returns list of runtime.Object
	// Returns error if objects cannot be listed otherwise returns nil.
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) ([]runtime.Object, error)
}

func NewK8sClient(cli client.Client, scheme *runtime.Scheme) *K8sClientWrapper {
	return &K8sClientWrapper{cli: cli, scheme: scheme}
}

type K8sClientWrapper struct {
	cli    client.Client
	scheme *runtime.Scheme
}

func (k K8sClientWrapper) Sync(
	ctx context.Context,
	blueprint client.Object,
	owner metav1.Object,
	diffOpts ...cmp.Option,
) (bool, error) {
	// we will compare this object later with blueprint
	actual, err := k.scheme.New(blueprint.GetObjectKind().GroupVersionKind())
	if err != nil {
		return false, err
	}

	key := types.NamespacedName{
		Name:      blueprint.GetName(),
		Namespace: blueprint.GetNamespace(),
	}
	if exists, err := k.doGetIgnoreNotFound(ctx, key, actual.(client.Object)); err != nil {
		return false, err
	} else if !exists {
		return k.doSync(ctx, nil, blueprint, owner, diffOpts...)
	}

	return k.doSync(ctx, actual.(client.Object), blueprint, owner, diffOpts...)
}

func (k K8sClientWrapper) Create(ctx context.Context, blueprint client.Object, owner metav1.Object) (bool, error) {
	return k.doCreate(ctx, blueprint, owner, false)
}

func (k K8sClientWrapper) CreateIgnoreIfExists(ctx context.Context, blueprint client.Object, owner metav1.Object) (bool, error) {
	return k.doCreate(ctx, blueprint, owner, true)
}

func (k K8sClientWrapper) Get(ctx context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return k.doGetIgnoreNotFound(ctx, key, objectMeta)
}

func (k K8sClientWrapper) GetClusterScoped(ctx context.Context, name string, objectMeta client.Object) (bool, error) {
	return k.doGetIgnoreNotFound(ctx, types.NamespacedName{Name: name}, objectMeta)
}

func (k K8sClientWrapper) Delete(ctx context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	return k.deleteByKeyIgnoreNotFound(ctx, key, objectMeta)
}

func (k K8sClientWrapper) DeleteClusterScoped(ctx context.Context, name string, objectMeta client.Object) (bool, error) {
	return k.deleteByKeyIgnoreNotFound(ctx, types.NamespacedName{Name: name}, objectMeta)
}

func (k K8sClientWrapper) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) ([]runtime.Object, error) {
	err := k.cli.List(ctx, list, opts...)
	if err != nil {
		return []runtime.Object{}, err
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return []runtime.Object{}, err
	}

	for i, _ := range items {
		if gvk, err := apiutil.GVKForObject(items[i], k.scheme); err != nil {
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
func (k K8sClientWrapper) deleteByKeyIgnoreNotFound(ctx context.Context, key client.ObjectKey, objectMeta client.Object) (bool, error) {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return false, fmt.Errorf("object %T is not a runtime.Object", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject().(client.Object)
	if exists, err := k.doGetIgnoreNotFound(ctx, key, actual); !exists {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return k.doDeleteIgnoreIfNotFound(ctx, actual)
}

// doDeleteIgnoreIfNotFound deletes object.
// Returns true if object deleted or not found otherwise returns false.
// Returns error if object cannot be deleted otherwise returns nil.
func (k K8sClientWrapper) doDeleteIgnoreIfNotFound(ctx context.Context, object client.Object) (bool, error) {
	if err := k.cli.Delete(ctx, object); err == nil {
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
func (k K8sClientWrapper) doGetIgnoreNotFound(ctx context.Context, key client.ObjectKey, object client.Object) (bool, error) {
	if err := k.cli.Get(ctx, key, object); err == nil {
		if err := k.ensureGVK(object); err != nil {
			return false, err
		}

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
func (k K8sClientWrapper) doCreate(ctx context.Context, blueprint client.Object, owner metav1.Object, ignoreIfAlreadyExists bool,
) (bool, error) {
	if err := k.setOwner(blueprint, owner); err != nil {
		return false, err
	}

	if err := k.cli.Create(ctx, blueprint); err == nil {
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
func (k K8sClientWrapper) doSync(
	ctx context.Context,
	actual client.Object,
	blueprint client.Object,
	owner metav1.Object,
	diffOpts ...cmp.Option,
) (bool, error) {
	if actual == nil {
		return k.doCreate(ctx, blueprint, owner, false)
	}

	// set GroupVersionKind (it might be empty)
	actual.GetObjectKind().SetGroupVersionKind(blueprint.GetObjectKind().GroupVersionKind())

	diff := cmp.Diff(actual, blueprint, diffOpts...)
	if len(diff) > 0 {
		// don't print difference if there are no diffOpts mainly to avoid huge output
		if len(diffOpts) != 0 {
			fmt.Printf("Difference:\n%s", diff)
		}

		if k.isRecreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			if done, err := k.doDeleteIgnoreIfNotFound(ctx, actual); !done {
				return false, err
			}
			return k.doCreate(ctx, blueprint, owner, false)
		} else {
			if err := k.setOwner(blueprint, owner); err != nil {
				return false, err
			}

			// to be able to update, we need to set the resource version of the object that we know of
			blueprint.(metav1.Object).SetResourceVersion(actual.GetResourceVersion())

			err := k.cli.Update(ctx, blueprint)
			if err == nil {
				logger.Info("Object updated", "namespace", actual.GetNamespace(), "kind", GetObjectType(actual), "name", actual.GetName())
			}
			return false, err
		}
	}

	return true, nil
}

// setOwner sets owner to the object
func (k K8sClientWrapper) setOwner(blueprint client.Object, owner metav1.Object) error {
	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, blueprint, k.scheme); err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	return nil
}

// isRecreate returns true, if object should be deleted/created instead of being updated.
func (k K8sClientWrapper) isRecreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind
}

func (k K8sClientWrapper) ensureGVK(obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, k.scheme)
	if err != nil {
		return err
	}

	obj.GetObjectKind().SetGroupVersionKind(gvk)
	return nil
}

func GetObjectType(obj interface{}) string {
	objType := reflect.TypeOf(obj).String()
	if reflect.TypeOf(obj).Kind().String() == "ptr" {
		objType = objType[1:]
	}

	return objType
}
