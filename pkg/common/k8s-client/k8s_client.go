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
	// Object is created if it does not exist and updated if it exists but is different.
	// Returns nil if object is in sync.
	Sync(ctx context.Context, blueprint client.Object, owner metav1.Object, diffOpts ...cmp.Option) error
	// Create creates object.
	// Returns nil if object is created otherwise returns error.
	Create(ctx context.Context, blueprint client.Object, owner metav1.Object, opts ...client.CreateOption) error
	// GetIgnoreNotFound gets object.
	// Returns true if object exists otherwise returns false.
	// Returns nil if object is retrieved or not found otherwise returns error.
	GetIgnoreNotFound(ctx context.Context, key client.ObjectKey, objectMeta client.Object, opts ...client.GetOption) (bool, error)
	// DeleteByKeyIgnoreNotFound deletes object by key.
	// Returns nil if object is deleted or not found otherwise returns error.
	DeleteByKeyIgnoreNotFound(ctx context.Context, key client.ObjectKey, objectMeta client.Object, opts ...client.DeleteOption) error
	// List returns list of runtime objects.
	// Returns nil if list is retrieved otherwise returns error.
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
	obj client.Object,
	owner metav1.Object,
	diffOpts ...cmp.Option,
) error {
	defer func() {
		// ensure GVK is set (for original object) when function returns
		_ = k.ensureGVK(obj)
	}()

	if err := k.ensureGVK(obj); err != nil {
		return err
	}

	if err := k.setOwner(obj, owner); err != nil {
		return err
	}

	actual, err := k.scheme.New(obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return err
	}

	key := types.NamespacedName{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	if exists, err := k.doGetIgnoreNotFound(ctx, key, actual.(client.Object)); exists {
		return k.doSync(ctx, actual.(client.Object), obj, diffOpts...)
	} else if err == nil {
		return k.doSync(ctx, nil, obj, diffOpts...)
	} else {
		return err
	}
}

func (k K8sClientWrapper) Create(
	ctx context.Context,
	obj client.Object,
	owner metav1.Object,
	opts ...client.CreateOption,
) error {
	defer func() {
		// ensure GVK is set (for original object) when function returns
		_ = k.ensureGVK(obj)
	}()

	if err := k.ensureGVK(obj); err != nil {
		return err
	}

	if err := k.setOwner(obj, owner); err != nil {
		return err
	}

	return k.doCreate(ctx, obj, false, opts...)
}

func (k K8sClientWrapper) GetIgnoreNotFound(
	ctx context.Context,
	key client.ObjectKey,
	objectMeta client.Object,
	opts ...client.GetOption,
) (bool, error) {
	return k.doGetIgnoreNotFound(ctx, key, objectMeta, opts...)
}

func (k K8sClientWrapper) DeleteByKeyIgnoreNotFound(
	ctx context.Context,
	key client.ObjectKey,
	objectMeta client.Object,
	opts ...client.DeleteOption,
) error {
	return k.deleteByKeyIgnoreNotFound(ctx, key, objectMeta, opts...)
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

// deleteByKeyIgnoreNotFound deletes object.
// Returns nil if object is deleted or not found otherwise returns error.
func (k K8sClientWrapper) deleteByKeyIgnoreNotFound(
	ctx context.Context,
	key client.ObjectKey,
	objectMeta client.Object,
	opts ...client.DeleteOption,
) error {
	runtimeObject, ok := objectMeta.(runtime.Object)
	if !ok {
		return fmt.Errorf("object %T is not a runtime.Object", runtimeObject)
	}

	actual := runtimeObject.DeepCopyObject().(client.Object)
	if exists, err := k.doGetIgnoreNotFound(ctx, key, actual); exists {
		return k.doDeleteIgnoreIfNotFound(ctx, actual, opts...)
	} else if err == nil {
		return nil
	} else {
		return err
	}
}

// doDeleteIgnoreIfNotFound deletes object.
// Returns nil if object is deleted or not found otherwise returns error.
func (k K8sClientWrapper) doDeleteIgnoreIfNotFound(
	ctx context.Context,
	obj client.Object,
	opts ...client.DeleteOption,
) error {
	if err := k.cli.Delete(ctx, obj, opts...); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Object not found", "namespace", obj.GetNamespace(), "kind", GetObjectType(obj), "name", obj.GetName())
			return nil
		} else {
			return err
		}
	}

	logger.Info("Object deleted", "namespace", obj.GetNamespace(), "kind", GetObjectType(obj), "name", obj.GetName())
	return nil
}

// doGetIgnoreNotFound gets object.
// Returns true if object exists otherwise returns false.
// Returns nil if object is retrieved or not found otherwise returns error.
func (k K8sClientWrapper) doGetIgnoreNotFound(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) (bool, error) {
	if err := k.cli.Get(ctx, key, obj, opts...); err == nil {
		if err = k.ensureGVK(obj); err != nil {
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
// Returns nil if object is created otherwise returns error.
func (k K8sClientWrapper) doCreate(
	ctx context.Context,
	obj client.Object,
	ignoreIfAlreadyExists bool,
	opts ...client.CreateOption,
) error {
	if err := k.cli.Create(ctx, obj, opts...); err == nil {
		logger.Info("Object created", "namespace", obj.GetNamespace(), "kind", GetObjectType(obj), "name", obj.GetName())
		return nil
	} else if errors.IsAlreadyExists(err) {
		if ignoreIfAlreadyExists {
			logger.Info("Object already exists, ignoring", "namespace", obj.GetNamespace(), "kind", GetObjectType(obj), "name", obj.GetName())
			return nil
		} else {
			return err
		}
	} else {
		return err
	}
}

// doSync ensures that the object is up to date in the cluster.
// Returns nil if object is in sync.
func (k K8sClientWrapper) doSync(
	ctx context.Context,
	actual client.Object,
	obj client.Object,
	diffOpts ...cmp.Option,
) error {
	if actual == nil {
		return k.doCreate(ctx, obj, false)
	}

	diff := cmp.Diff(actual, obj, diffOpts...)
	if len(diff) > 0 {
		// don't print difference if there are no diffOpts mainly to avoid huge output
		if len(diffOpts) != 0 {
			fmt.Printf("Difference:\n%s", diff)
		}

		if k.isRecreate(actual.GetObjectKind().GroupVersionKind().Kind) {
			if err := k.doDeleteIgnoreIfNotFound(ctx, actual); err != nil {
				return err
			}

			return k.doCreate(ctx, obj, false)
		} else {
			// to be able to update, we need to set the resource version of the object that we know of
			obj.(metav1.Object).SetResourceVersion(actual.GetResourceVersion())

			err := k.cli.Update(ctx, obj)
			if err == nil {
				logger.Info("Object updated", "namespace", actual.GetNamespace(), "kind", GetObjectType(actual), "name", actual.GetName())
			}
			return err
		}
	}

	return nil
}

// setOwner sets owner to the object
func (k K8sClientWrapper) setOwner(obj client.Object, owner metav1.Object) error {
	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, obj, k.scheme); err != nil {
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
