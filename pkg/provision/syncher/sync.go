//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package syncher

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Package syncher provides low-level utilities for provisioning, synchronizing,
// and cleaning up Kubernetes resources. It serves as a higher-level abstraction
// layer in front of K8sClientWrapper, simplifying cluster interaction calls.

// SyncObject ensures that the object is up to date in the cluster.
// Besides the actual sync it:
//   - selects the client to use: the cached one for objects labeled with
//     `app.kubernetes.io/part-of: che.eclipse.org`; see `getCacheFunc` in `cmd/main.go`.
//   - sets the CheCluster as the controller owner if the object lives in the same namespace as the CR.
//   - builds the sync options: existing labels and annotations are merged rather than replaced,
//     and only the labels and annotations defined on the given object participate in the diff,
//     together with the controller owner reference and the finalizers (for the CheCluster itself).
//     This applies to a blueprint built from scratch (no resourceVersion). An object carrying a
//     resourceVersion (read from the cluster) is written as is: nothing is merged and all its
//     labels and annotations are compared, so keys removed from it are removed from the cluster too.
//   - retries the sync with recreation if the update failed because an immutable field has changed,
//     but only when `allowForceRecreate` is set.
func SyncObject(
	cheCtx *chetypes.CheContext,
	obj client.Object,
	diff cmp.Options,
	suppressDiff bool,
	allowForceRecreate bool) error {

	cw := getClientFor(cheCtx, obj)

	if err := setOwnerReferenceIfNeeded(cheCtx, obj); err != nil {
		return err
	}

	// The operator fully owns the finalizers of the CheCluster itself, so they are synced
	// and compared. On every other object, finalizers set by other controllers must be
	// preserved and kept out of the comparison.
	preserveFinalizers := !isCheCluster(obj)

	// An object carrying a resourceVersion was read from the cluster (or returned by a previous
	// update), and the client wrapper writes it as is, without merging the existing metadata into
	// it. All its labels and annotations are compared then, otherwise a key removed on purpose
	// would not be detected and the object would not be updated.
	compareAllMetadataKeys := obj.GetResourceVersion() != ""

	opts := &clientwrapper.SyncOptions{
		MergeAnnotations:   true,
		MergeLabels:        true,
		PreserveFinalizers: preserveFinalizers,
		SuppressDiff:       suppressDiff,
		DiffOpts: cmp.Options{
			diff,
			cmpMetadata(
				slices.Collect(maps.Keys(obj.GetLabels())),
				slices.Collect(maps.Keys(obj.GetAnnotations())),
				compareAllMetadataKeys,
				preserveFinalizers,
			),
		},
	}

	if err := cw.Sync(cheCtx.Context, obj, opts); err != nil {
		if isFieldImmutableError(err) && allowForceRecreate {
			opts.ForceRecreate = true

			// the failed update left the resource version of the existing object on `obj`,
			// it has to be dropped, otherwise the API server rejects the subsequent create
			obj.SetResourceVersion("")

			if err := cw.Sync(cheCtx.Context, obj, opts); err != nil {
				return getSyncError(obj, err)
			}

			return nil
		}

		return getSyncError(obj, err)
	}

	return nil
}

// Create creates the object in the cluster. It fails if the object already exists -
// use `SyncObject` instead when the object must be created or updated to the desired state.
//
// It selects the client like `SyncObject` does, sets the CheCluster as the controller
// owner if the object lives in the same namespace as the CR, and drops the resource
// version so that a blueprint reused across reconciles is always accepted by the API server.
func Create(cheCtx *chetypes.CheContext, obj client.Object) error {
	cw := getClientFor(cheCtx, obj)

	if err := setOwnerReferenceIfNeeded(cheCtx, obj); err != nil {
		return err
	}

	obj.SetResourceVersion("")

	if err := cw.Create(cheCtx.Context, obj); err != nil {
		return getCreateError(obj, err)
	}

	return nil
}

// SyncObjectStatus ensures that the status subresource of the object is up to date in the cluster.
// It mirrors `SyncObject` but syncs only the status: the spec and the metadata are not touched.
// The object must already exist, otherwise an error is returned.
func SyncObjectStatus(
	cheCtx *chetypes.CheContext,
	obj client.Object,
	diff cmp.Options,
	suppressDiff bool) error {

	cw := getClientFor(cheCtx, obj)

	opts := &clientwrapper.SyncOptions{
		SuppressDiff:  suppressDiff,
		DiffOpts:      cmp.Options{diff},
		ForceRecreate: false,
	}

	err := cw.SyncStatus(cheCtx.Context, obj, opts)
	if err != nil {
		return getSyncError(obj, err)
	}

	return nil
}

// GetObject fetches the object with the given name and namespace into obj.
// It returns false without an error if the object does not exist.
//
// It always uses the cached ClientWrapper, and the operator's cache only watches
// objects labeled with `app.kubernetes.io/part-of: che.eclipse.org` (see `getCacheFunc`
// in `cmd/main.go`). So it is only correct for objects the operator manages or has
// backfilled; for arbitrary objects the non-caching client must be used directly.
// Note that unlike `SyncObject`, the client cannot be selected from the object's
// labels here: they are only known after the object is fetched.
func GetObject(
	cheCtx *chetypes.CheContext,
	name string,
	namespace string,
	obj client.Object,
	opts ...client.GetOption,
) (bool, error) {
	cw := cheCtx.ClusterAPI.ClientWrapper
	key := types.NamespacedName{Name: name, Namespace: namespace}

	exists, err := cw.GetIgnoreNotFound(cheCtx.Context, key, obj, opts...)
	if err != nil {
		return false, fmt.Errorf("failed to get %s %s: %w", clientwrapper.GetObjectType(obj), key.String(), err)
	}

	return exists, nil
}

// DeleteObject ensures that the object is deleted in the cluster.
//
// It always uses the cached ClientWrapper, and the operator's cache only watches
// objects labeled with `app.kubernetes.io/part-of: che.eclipse.org` (see `getCacheFunc`
// in `cmd/main.go`). So it is only correct for objects the operator manages or has
// backfilled; for arbitrary objects the non-caching client must be used directly.
func DeleteObject(
	cheCtx *chetypes.CheContext,
	name string,
	namespace string,
	obj client.Object,
	opts ...client.DeleteOption,
) error {
	cw := cheCtx.ClusterAPI.ClientWrapper
	key := types.NamespacedName{Name: name, Namespace: namespace}

	if err := cw.DeleteByKeyIgnoreNotFound(cheCtx.Context, key, obj, opts...); err != nil {
		return fmt.Errorf("failed to delete %s %s: %w", clientwrapper.GetObjectType(obj), key.String(), err)
	}

	return nil
}

// DeleteAllManaged ensures that all objects of the given type managed by the operator
// for the specified component are deleted in the cluster.
// If namespace is not empty, the deletion is scoped to that namespace.
func DeleteAllManaged(
	cheCtx *chetypes.CheContext,
	namespace string,
	obj client.Object,
	component string,
) error {
	cw := cheCtx.ClusterAPI.ClientWrapper
	cheLabels := chelabels.GetOperatorManagedLabels(component)

	opts := &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			LabelSelector: labels.SelectorFromSet(cheLabels),
		},
	}

	if namespace != "" {
		opts.Namespace = namespace
	}

	if err := cw.DeleteAllOf(cheCtx.Context, obj, opts); err != nil {
		if namespace == "" {
			return fmt.Errorf("failed to delete %s: %w", clientwrapper.GetObjectType(obj), err)
		}
		return fmt.Errorf("failed to delete %s in %s: %w", clientwrapper.GetObjectType(obj), namespace, err)
	}

	return nil
}

// cmpMetadata compares only the specified labels and annotations, finalizers (unless
// preserveFinalizers is true), and the controller owner reference. All other metadata is
// ignored. Finalizers are compared as a set, since their order is not significant.
//
// If compareAllKeys is true, labelKeys and annotationKeys are ignored and all labels and
// annotations are compared, so that a key present only on one side is detected as well.
//
// It is applied as a metav1.ObjectMeta transformer via cmp.FilterPath. It only works
// with types embedding metav1.ObjectMeta at the top level; it has no effect on
// *unstructured.Unstructured. Using a transformer ensures diffs show only the
// compared metadata fields.
func cmpMetadata(
	labelKeys []string,
	annotationKeys []string,
	compareAllKeys bool,
	preserveFinalizers bool,
) cmp.Option {
	transform := cmp.Transformer("cmpMetadata", func(m metav1.ObjectMeta) comparableMetadata {
		labels := selectEntries(m.Labels, labelKeys, compareAllKeys)
		annotations := selectEntries(m.Annotations, annotationKeys, compareAllKeys)

		var finalizers []string
		if !preserveFinalizers {
			finalizers = slices.Sorted(slices.Values(m.Finalizers))
		}

		return comparableMetadata{
			Labels:      labels,
			Annotations: annotations,
			Finalizers:  finalizers,
			Controller:  metav1.GetControllerOf(&m),
		}
	})

	return cmp.FilterPath(isRootMetadata, transform)
}

func getClientFor(cheCtx *chetypes.CheContext, obj client.Object) clientwrapper.ClientWrapper {
	if isCheCluster(obj) || obj.GetLabels()[constants.KubernetesPartOfLabelKey] == constants.CheEclipseOrg {
		return cheCtx.ClusterAPI.ClientWrapper
	}

	return cheCtx.ClusterAPI.NonCachingClientWrapper
}

func setOwnerReferenceIfNeeded(cheCtx *chetypes.CheContext, obj client.Object) error {
	if metav1.GetControllerOf(obj) != nil {
		return nil
	}

	if cheCtx.CheCluster == nil || isCheCluster(obj) {
		return nil
	}

	if obj.GetNamespace() != cheCtx.CheCluster.Namespace {
		return nil
	}

	return controllerutil.SetControllerReference(cheCtx.CheCluster, obj, cheCtx.ClusterAPI.Scheme)
}

// selectEntries returns a new map with the entries of m for the given keys; a key missing
// from m maps to an empty value. If allKeys is true, all entries of m are returned instead.
// The result is never nil, so that a nil map and an empty map compare as equal.
func selectEntries(m map[string]string, keys []string, allKeys bool) map[string]string {
	if allKeys {
		result := make(map[string]string, len(m))
		maps.Copy(result, m)

		return result
	}

	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = m[key]
	}

	return result
}

// comparableMetadata is the reduced view of `metav1.ObjectMeta` that `cmpMetadata` compares.
type comparableMetadata struct {
	Labels      map[string]string
	Annotations map[string]string
	Finalizers  []string
	Controller  *metav1.OwnerReference
}

// isRootMetadata reports whether p points at the top-level ObjectMeta.
func isRootMetadata(p cmp.Path) bool {
	if _, ok := p.Last().(cmp.StructField); !ok {
		return false
	}

	for _, step := range p[:len(p)-1] {
		if _, ok := step.(cmp.StructField); ok {
			return false
		}
	}

	return true
}

// isFieldImmutableError reports whether the API server rejected an update because it
// touched an immutable field. Kubernetes exposes no structured signal for this, so the
// message has to be matched; `validation.FieldImmutableErrorMsg` covers the generic
// `ValidateImmutableField` path used by ConfigMap, Secret, Job, Service.ClusterIP and
// Deployment.Selector.
func isFieldImmutableError(err error) bool {
	return apierrors.IsInvalid(err) && strings.Contains(err.Error(), validation.FieldImmutableErrorMsg)
}

func getCreateError(obj client.Object, cause error) error {
	return fmt.Errorf("failed to create %s %s/%s: %w", clientwrapper.GetObjectType(obj), obj.GetNamespace(), obj.GetName(), cause)
}

func getSyncError(obj client.Object, cause error) error {
	return fmt.Errorf("failed to sync %s %s/%s: %w", clientwrapper.GetObjectType(obj), obj.GetNamespace(), obj.GetName(), cause)
}

func isCheCluster(obj client.Object) bool {
	_, ok := obj.(*chev2.CheCluster)
	return ok
}
