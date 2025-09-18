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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// MergeLabelsAnnotationsFromClusterObject adds labels and annotations from the cluster object
// to the provided object if they do not already exist on it.
func MergeLabelsAnnotationsFromClusterObject(ctx context.Context, scheme *runtime.Scheme, cli client.Client, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return err
	}

	actual, err := scheme.New(gvk)
	if err != nil {
		return err
	}

	if err := cli.Get(ctx, client.ObjectKeyFromObject(obj), actual.(client.Object)); err == nil {
		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(map[string]string{})
		}

		for k, v := range actual.(client.Object).GetAnnotations() {
			if _, ok := obj.GetAnnotations()[k]; !ok {
				obj.GetAnnotations()[k] = v
			}
		}

		if obj.GetLabels() == nil {
			obj.SetLabels(map[string]string{})
		}

		for k, v := range actual.(client.Object).GetLabels() {
			if _, ok := obj.GetLabels()[k]; !ok {
				obj.GetLabels()[k] = v
			}
		}
	} else {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}
