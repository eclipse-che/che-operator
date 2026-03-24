//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package infrastructure

import (
	"os"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Type int

const (
	Unknown Type = iota
	Kubernetes
	OpenShiftV4

	LeasesResources                = "leases"
	OAuthClientsResources          = "oauthclients"
	KubernetesImagePullerResources = "kubernetesimagepullers"
)

var (
	infrastructure = Unknown

	isOpenShiftOAuthEnabled        bool
	isLeaderElectionEnabled        bool
	isKubernetesImagePullerEnabled bool

	logger = ctrl.Log.WithName("infrastructure")
)

func GetOperatorNamespace() (string, error) {
	nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}

	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

func IsOpenShift() bool {
	initializeIfNeeded()
	return infrastructure == OpenShiftV4
}

func IsOpenShiftOAuthEnabled() bool {
	initializeIfNeeded()
	return isOpenShiftOAuthEnabled
}

func IsLeaderElectionEnabled() bool {
	initializeIfNeeded()
	return isLeaderElectionEnabled
}

func IsKubernetesImagePullerEnabled() bool {
	initializeIfNeeded()
	return isKubernetesImagePullerEnabled
}

func InitializeForTesting(desiredInfrastructure Type) {
	infrastructure = desiredInfrastructure

	if IsOpenShift() {
		isOpenShiftOAuthEnabled = true
	} else {
		isOpenShiftOAuthEnabled = false
	}

	isKubernetesImagePullerEnabled = true
	isLeaderElectionEnabled = true
}

func initializeIfNeeded() {
	if infrastructure != Unknown {
		return
	}

	kubeCfg, err := config.GetConfig()
	if err != nil {
		panic("Failed to get kubeconfig")
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeCfg)
	if err != nil {
		panic("Failed to create discovery client")
	}

	apiGroups, apiResources, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		panic("Failed to get API Groups and Resources")
	}

	if hasAPIGroup(apiGroups, "config.openshift.io") {
		infrastructure = OpenShiftV4
		isOpenShiftOAuthEnabled = hasAPIResource(apiResources, OAuthClientsResources)
	} else {
		infrastructure = Kubernetes
		isOpenShiftOAuthEnabled = false
	}

	isLeaderElectionEnabled = hasAPIResource(apiResources, LeasesResources)
	isKubernetesImagePullerEnabled = hasAPIResource(apiResources, KubernetesImagePullerResources)
}

func hasAPIGroup(source []*metav1.APIGroup, apiName string) bool {
	return slices.ContainsFunc(source, func(g *metav1.APIGroup) bool {
		return g.Name == apiName
	})
}

func hasAPIResource(resources []*metav1.APIResourceList, resourceName string) bool {
	for _, resource := range resources {
		for _, r := range resource.APIResources {
			if r.Name == resourceName {
				return true
			}
		}
	}

	return false
}
