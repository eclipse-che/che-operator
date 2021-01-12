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
package util

import (
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
	_, err := fakeK8s.clientset.CoreV1().Pods(namespace).Create(&corev1.Pod{
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
	})
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

func TestGetEvents(t *testing.T) {

	// fire up an event with fake-pod as involvedObject
	message := "This is a fake event about a fake pod"
	_, err := fakeK8s.clientset.CoreV1().Events(namespace).Create(&corev1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-event",
			Namespace: "eclipse-che",
		},
		InvolvedObject: corev1.ObjectReference{
			FieldPath: "spec.containers{fake-pod}",
			Kind:      "Pod",
		},
		Message: message,
		Reason:  "Testing event filtering",
		Type:    "Normal",
	})

	if err != nil {
		panic(err)
	}

	events := fakeK8s.GetEvents("fake-pod", namespace)
	fakePodEvents := events.Items
	if len(fakePodEvents) == 0 {
		logrus.Fatal("Test failed No events found")
	} else {
		logrus.Infof("Test passed. Found %v event", len(fakePodEvents))
	}
	// test if event message matches
	fakePodEventMessage := events.Items[0].Message
	if len(fakePodEventMessage) != len(message) {
		t.Errorf("Test failed. Message to be received: %s, but got %s ", message, fakePodEventMessage)
	} else {
		logrus.Infof("Test passed. Expected event message: %s. Received event message %s", message, fakePodEventMessage)
	}
}
