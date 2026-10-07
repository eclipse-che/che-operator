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

package provision

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SyncObject ensures that the object is up to date in the cluster.
// Besides the actual sync it:
//   - selects the client to use: the cached one for objects labeled with
//     `app.kubernetes.io/part-of: che.eclipse.org` (they are watched by the operator),
//     the non-caching one otherwise; see `cmd/main.go:365`.
//   - sets the CheCluster as the controller owner if the object lives in the same namespace as the CR
//     (cross-namespace owner references are not supported by Kubernetes);
//   - builds the sync options: existing labels and annotations are merged rather than replaced,
//     and only the labels and annotations defined on the given object participate in the diff;
//     should not be used with `pkg/common/diffs/diffs.go`.
//   - retries the sync with recreation if the update failed because an immutable field has changed.
func SyncObject(
	cheCtx *chetypes.CheContext,
	obj client.Object,
	diff cmp.Options,
	suppressDiff bool,
	allowForceRecreate bool) error {

	var cw *k8sclient.K8sClientWrapper

	if obj.GetLabels()[constants.KubernetesPartOfLabelKey] == constants.CheEclipseOrg {
		cw = cheCtx.ClusterAPI.ClientWrapper
	} else {
		cw = cheCtx.ClusterAPI.NonCachingClientWrapper
	}

	if cheCtx.CheCluster != nil {
		if obj.GetNamespace() == cheCtx.CheCluster.Namespace {
			if err := controllerutil.SetControllerReference(cheCtx.CheCluster, obj, cheCtx.ClusterAPI.Scheme); err != nil {
				return getSyncError(obj, err)
			}
		}
	}

	opts := &k8sclient.SyncOptions{
		MergeAnnotations: true,
		MergeLabels:      true,
		SuppressDiff:     suppressDiff,
		DiffOpts: cmp.Options{
			diff,
			cmpMetadata(
				slices.Collect(maps.Keys(obj.GetLabels())),
				slices.Collect(maps.Keys(obj.GetAnnotations())),
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

func cmpMetadata(
	labelKeys []string,
	annotationKeys []string,
) cmp.Option {
	return cmp.FilterPath(isRootMetadata, cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		for _, key := range labelKeys {
			if x.Labels[key] != y.Labels[key] {
				return false
			}
		}

		for _, key := range annotationKeys {
			if x.Annotations[key] != y.Annotations[key] {
				return false
			}
		}

		return equality.Semantic.DeepEqual(
			metav1.GetControllerOf(&x),
			metav1.GetControllerOf(&y),
		)
	}))
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

func getSyncError(obj client.Object, cause error) error {
	return fmt.Errorf("failed to sync %s %s/%s: %w", k8sclient.GetObjectType(obj), obj.GetNamespace(), obj.GetName(), cause)
}
