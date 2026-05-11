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
	"maps"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	securityv1 "github.com/openshift/api/security/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var Role = cmp.Options{
	cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta", "ObjectMeta"),
}

var RoleBinding = cmp.Options{
	cmpopts.IgnoreFields(rbac.RoleBinding{}, "TypeMeta", "ObjectMeta"),
}

var ClusterRole = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRole{}, "TypeMeta", "ObjectMeta"),
}

var ClusterRoleBinding = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRoleBinding{}, "TypeMeta", "ObjectMeta"),
}

var SecurityContextConstraints = cmp.Options{
	cmpopts.IgnoreFields(securityv1.SecurityContextConstraints{}, "TypeMeta", "ObjectMeta", "Priority"),
}

var ConfigMapAllLabels = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return maps.Equal(x.Labels, y.Labels)
	}),
}

func ConfigMap(labels []string, annotations []string) cmp.Options {
	return cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
		objectMetaComparator(labels, annotations),
	}
}

var ServiceMonitor = cmp.Options{
	cmpopts.IgnoreFields(monitoringv1.ServiceMonitor{}, "TypeMeta", "ObjectMeta"),
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
