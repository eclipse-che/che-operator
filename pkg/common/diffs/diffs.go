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
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	securityv1 "github.com/openshift/api/security/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
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

var ConfigMapEnsureLabels = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return maps.Equal(x.Labels, y.Labels)
	}),
}

var Service = cmp.Options{
	cmpopts.IgnoreFields(corev1.Service{}, "TypeMeta", "ObjectMeta", "Status"),
	cmp.Comparer(func(x, y corev1.ServiceSpec) bool {
		return maps.Equal(x.Selector, y.Selector) && reflect.DeepEqual(x.Ports, y.Ports)
	}),
}

func Ingress(labels []string, annotations []string) cmp.Options {
	return cmp.Options{
		cmpopts.IgnoreFields(networking.Ingress{}, "TypeMeta", "Status"),
		cmpMetadata(labels, annotations),
	}
}

var Job = cmp.Options{
	cmpopts.IgnoreFields(batchv1.Job{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(batchv1.JobSpec{}, "Selector", "TTLSecondsAfterFinished"),
	cmpopts.IgnoreFields(corev1.PodTemplateSpec{}, "ObjectMeta"),
	cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy"),
	cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "DeprecatedServiceAccount"),
	cmpopts.IgnoreFields(corev1.ConfigMapVolumeSource{}, "DefaultMode"),
	cmpopts.IgnoreFields(corev1.SecretVolumeSource{}, "DefaultMode"),
}

// ConfigMap respects existed labels and annotations
func ConfigMap(labelKeys []string, annotationKeys []string) cmp.Options {
	return cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
		cmpMetadata(labelKeys, annotationKeys),
	}
}

var ServiceMonitor = cmp.Options{
	cmpopts.IgnoreFields(monitoringv1.ServiceMonitor{}, "TypeMeta", "ObjectMeta"),
}

func cmpMetadata(labels []string, annotations []string) cmp.Option {
	return cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		for _, label := range labels {
			if x.Labels[label] != y.Labels[label] {
				return false
			}
		}

		for _, annotation := range annotations {
			if x.Annotations[annotation] != y.Annotations[annotation] {
				return false
			}
		}

		return true
	})
}
