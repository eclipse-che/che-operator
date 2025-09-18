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

package diffs

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ConfigMapAll = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels) && reflect.DeepEqual(x.Annotations, y.Annotations)
	}),
}

var ConfigMapAllLabels = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels)
	}),
}

func ConfigMap(labels []string, annotations []string) cmp.Options {
	return cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
		objectMetaComparator(labels, annotations),
	}
}

func objectMetaComparator(labels []string, annotations []string) cmp.Option {
	return cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		if labels != nil {
			for _, label := range labels {
				if x.Labels[label] != y.Labels[label] {
					return false
				}
			}
		}

		if annotations != nil {
			for _, annotation := range annotations {
				if x.Annotations[annotation] != y.Annotations[annotation] {
					return false
				}
			}
		}

		return true
	})
}
