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
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type K8sHelper struct {
	clientset kubernetes.Interface
	client    client.Client
}

var (
	k8sHelper *K8sHelper
	logger    = ctrl.Log.WithName("k8sHelper")
)

func New() *K8sHelper {
	if k8sHelper != nil {
		return k8sHelper
	}

	if isTestMode() {
		return initializeForTesting()
	}

	if err := initialize(); err != nil {
		logger.Error(err, "Failed to initialize Kubernetes client")
		os.Exit(1)
	}

	return k8sHelper
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
		clientset: fake.NewClientset(),
		client:    fakeclient.NewClientBuilder().Build(),
	}

	k8sHelper.clientset.Discovery().(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: "devworkspaceoperatorconfigs"},
			},
		},
	}

	return k8sHelper
}

func initialize() error {
	cfg := config.GetConfigOrDie()

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	client, err := client.New(cfg, client.Options{Scheme: runtime.NewScheme()})
	if err != nil {
		return err
	}

	k8sHelper = &K8sHelper{
		clientset: clientset,
		client:    client,
	}

	return nil
}
func isTestMode() bool {
	return len(os.Getenv("MOCK_API")) != 0
}
