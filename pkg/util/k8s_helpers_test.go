//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package util

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	fakeK8s   = fakeClientSet()
	namespace = "eclipse-che"
)

func fakeClientSet() *k8s {
	client := k8s{}
	client.clientset = fake.NewSimpleClientset()
	return &client
}

func TestGetDeploymentPod(t *testing.T) {

	// create a fake pod
	_, err := fakeK8s.clientset.CoreV1().Pods(namespace).Create(context.TODO(), &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-pod",
			Namespace: namespace,
			Labels: map[string]string{
				"component": "postgres",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
	pod, err := fakeK8s.GetDeploymentPod("postgres", namespace)
	if err != nil {
		t.Errorf("Failed to det deployment pod: %s", err)
	}
	if len(pod) == 0 {
		t.Errorf("Test failed. No pods found by label")
	}
	logrus.Infof("Test passed. Pod %s found", pod)
}
