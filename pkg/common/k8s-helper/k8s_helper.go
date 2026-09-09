//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type K8sHelper struct {
	client          client.Client
	clientSet       kubernetes.Interface
	discoveryClient discovery.DiscoveryInterface
}

var (
	k8sHelper *K8sHelper
)

func Initialize() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	client, err := client.New(cfg, client.Options{Scheme: runtime.NewScheme()})
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	k8sHelper = &K8sHelper{
		client:          client,
		clientSet:       clientSet,
		discoveryClient: clientSet.Discovery(),
	}

	return nil
}

func InitializeForTesting() {
	clientSet := fake.NewSimpleClientset()

	k8sHelper = &K8sHelper{
		clientSet:       clientSet,
		client:          fakeclient.NewClientBuilder().Build(),
		discoveryClient: clientSet.Discovery(),
	}
}

func GetInstance() *K8sHelper {
	if !isInitialized() {
		panic("Kubernetes helper is not initialized")
	}

	return k8sHelper
}

func (k *K8sHelper) GetClientSet() kubernetes.Interface {
	return k.clientSet
}

func (k *K8sHelper) GetClient() client.Client {
	return k.client
}

func (k *K8sHelper) GetDiscoveryClient() discovery.DiscoveryInterface {
	return k.discoveryClient
}

func (k *K8sHelper) GetPodsByComponent(name string, ns string) []string {
	names := []string{}
	api := k.clientSet.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "component=" + name,
	}
	podList, _ := api.Pods(ns).List(context.TODO(), listOptions)
	for _, pod := range podList.Items {
		names = append(names, pod.Name)
	}

	return names
}

func isInitialized() bool {
	return k8sHelper != nil
}
