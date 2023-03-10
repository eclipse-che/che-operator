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
package k8shelper

import (
	"context"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type K8sHelper struct {
	clientset kubernetes.Interface
	client    client.Client
}

var (
	k8sHelper *K8sHelper
)

func New() *K8sHelper {
	if k8sHelper != nil {
		return k8sHelper
	}

	if isTestMode() {
		return initializeForTesting()
	}

	return initialize()
}

func (cl *K8sHelper) GetClientset() kubernetes.Interface {
	return cl.clientset
}

func (cl *K8sHelper) GetClient() client.Client {
	return cl.client
}

func (cl *K8sHelper) GetPodsByComponent(name string, ns string) []string {
	names := []string{}
	api := cl.clientset.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "component=" + name,
	}
	podList, _ := api.Pods(ns).List(context.TODO(), listOptions)
	for _, pod := range podList.Items {
		names = append(names, pod.Name)
	}

	return names
}

func initializeForTesting() *K8sHelper {
	k8sHelper = &K8sHelper{
		clientset: fake.NewSimpleClientset(),
		client:    fakeclient.NewClientBuilder().Build(),
	}

	return k8sHelper
}

func initialize() *K8sHelper {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Fatalf("Failed to initialized Kubernetes client: %v", err)
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Failed to initialized Kubernetes client: %v", err)
	}

	client, err := client.New(cfg, client.Options{Scheme: runtime.NewScheme()})
	if err != nil {
		logrus.Fatalf("Failed to initialized Kubernetes client: %v", err)
	}

	k8sHelper = &K8sHelper{
		clientset: clientSet,
		client:    client,
	}

	return k8sHelper
}
func isTestMode() bool {
	return len(os.Getenv("MOCK_API")) != 0
}
