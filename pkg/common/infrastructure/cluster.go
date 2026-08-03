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
	"fmt"
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
	ServiceMonitorResources        = "servicemonitors"
)

var (
	infrastructure = Unknown

	isOpenShiftOAuthEnabled bool
	isLeaderElectionEnabled bool
	isServiceMonitorEnabled bool

	operatorNamespace string

	logger = ctrl.Log.WithName("infrastructure")
)

// GetOperatorNamespace returns the namespace where the operator is running.
// The result is cached to avoid repeated filesystem reads.
func GetOperatorNamespace() (string, error) {
	if operatorNamespace == "" {
		nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err == nil {
			operatorNamespace = strings.TrimSpace(string(nsBytes))
			return operatorNamespace, nil
		}

		// for the purpose of local run
		namespace, ok := os.LookupEnv("WATCH_NAMESPACE")
		if ok {
			operatorNamespace = namespace
			return operatorNamespace, nil
		}

		return "", fmt.Errorf("operator namespace is not set")
	}

	return operatorNamespace, nil
}

func IsOpenShift() bool {
	initializeIfNeeded()
	return infrastructure == OpenShiftV4
}

func IsOpenShiftOAuthEnabled() bool {
	initializeIfNeeded()
	return isOpenShiftOAuthEnabled
}

func IsOpenShiftExternalAuth() bool {
	initializeIfNeeded()
	return IsOpenShift() && !IsOpenShiftOAuthEnabled()
}

func IsLeaderElectionEnabled() bool {
	initializeIfNeeded()
	return isLeaderElectionEnabled
}

func IsKubernetesImagePullerEnabled(discovery discovery.DiscoveryInterface) bool {
	_, apiResources, err := discovery.ServerGroupsAndResources()
	if err != nil {
		logger.Error(err, "Failed to get API resources list")
		return false
	}

	return hasAPIResource(apiResources, KubernetesImagePullerResources)
}

func IsServiceMonitorEnabled() bool {
	initializeIfNeeded()
	return isServiceMonitorEnabled
}

func SetOpenShiftOAuthEnabledForTesting(enabled bool) {
	isOpenShiftOAuthEnabled = enabled
}

func InitializeForTesting(desiredInfrastructure Type) {
	infrastructure = desiredInfrastructure

	if IsOpenShift() {
		isOpenShiftOAuthEnabled = true
		operatorNamespace = "openshift-operators"
	} else {
		isOpenShiftOAuthEnabled = false
		operatorNamespace = "eclipse-che"
	}

	isLeaderElectionEnabled = true
	isServiceMonitorEnabled = true
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
	isServiceMonitorEnabled = hasAPIResource(apiResources, ServiceMonitorResources)
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
