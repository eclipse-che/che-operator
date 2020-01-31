//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestMergeServices(t *testing.T) {
	// given
	out := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{Name: "t1", Labels: map[string]string{"app": "che"}},
		Spec: corev1.ServiceSpec{
			ClusterIP: "123.123.123.123",
			Ports: []corev1.ServicePort{{Name: "p1",Port: 1}},
			Selector: map[string]string{"app": "che"},
		},
	}
	in := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{Name: "t2", Labels: map[string]string{"app": "che", "c": "d"}},
		Spec: corev1.ServiceSpec{
			ClusterIP: "1234",
			Ports: []corev1.ServicePort{{Name: "p2",Port: 2}, {Name: "p3", Port: 3}},
			Selector: map[string]string{"app": "che", "c": "d"},
		},
	}

	// when
	MergeServices(out, in)

	// then
	if out.Name != "t1" {
		t.Error("Name should not be updated")
	}
	if out.Spec.ClusterIP != "123.123.123.123" {
		t.Error("ClusterIp should not be updated")
	}
	if len(out.Labels) != 2 || out.Spec.Selector["app"] != "che" || out.Labels["c"] != "d" {
		t.Error("Labels should be updated")
	}
	if len(out.Spec.Ports) != 2 || out.Spec.Ports[0].Port != 2  || out.Spec.Ports[1].Port != 3 {
		t.Error("Port should be updated")
	}
	if len(out.Spec.Selector) != 2 || out.Spec.Selector["app"] != "che" || out.Spec.Selector["c"] != "d" {
		t.Error("Selector should be updated")
	}
}
