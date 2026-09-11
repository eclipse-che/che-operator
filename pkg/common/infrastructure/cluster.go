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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Type int

const (
	Unknown Type = iota
	Kubernetes
	OpenShiftV4
)

var (
	infrastructure = Unknown

	isOpenShiftOAuthEnabled bool
	isLeaderElectionEnabled bool
	isServiceMonitorEnabled bool

	operatorNamespace string

	agentSandbox          = schema.GroupKind{Group: "agents.x-k8s.io", Kind: "Sandbox"}
	serviceMonitor        = schema.GroupKind{Group: "monitoring.coreos.com", Kind: "ServiceMonitor"}
	kubernetesImagePuller = schema.GroupKind{Group: "che.eclipse.org", Kind: "KubernetesImagePuller"}
	oAuthClient           = schema.GroupKind{Group: "oauth.openshift.io", Kind: "OAuthClient"}
	leaseCoordination     = schema.GroupKind{Group: "coordination.k8s.io", Kind: "Lease"}

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

	return hasAPIResource(apiResources, kubernetesImagePuller)
}

func IsAgentSandboxEnabled(discovery discovery.DiscoveryInterface) bool {
	_, apiResources, err := discovery.ServerGroupsAndResources()
	if err != nil {
		logger.Error(err, "Failed to get API resources list")
		return false
	}

	return hasAPIResource(apiResources, agentSandbox)
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
		isOpenShiftOAuthEnabled = hasAPIResource(apiResources, oAuthClient)
	} else {
		infrastructure = Kubernetes
		isOpenShiftOAuthEnabled = false
	}

	isLeaderElectionEnabled = hasAPIResource(apiResources, leaseCoordination)
	isServiceMonitorEnabled = hasAPIResource(apiResources, serviceMonitor)
}

func hasAPIGroup(source []*metav1.APIGroup, apiName string) bool {
	return slices.ContainsFunc(source, func(g *metav1.APIGroup) bool {
		return g.Name == apiName
	})
}

// hasAPIResource checks if a resource with the given Group and Kind exists in the cluster.
// Handles both aggregated discovery (K8s 1.27+) and legacy discovery modes:
// - Aggregated discovery: apiResource.Group is explicitly populated
// - Legacy discovery: apiResource.Group is empty; group must be parsed from APIResourceList.GroupVersion
func hasAPIResource(apiResourcesLists []*metav1.APIResourceList, gk schema.GroupKind) bool {
	for _, apiResourcesList := range apiResourcesLists {
		// Parse the group from the list's GroupVersion field for legacy discovery mode.
		// This is needed because APIResource.Group may be empty in legacy discovery,
		// with the docs stating: "Empty implies the group of the containing resource list."
		apiResourcesListGroup := ""
		if apiResourcesList.GroupVersion != "" {
			gv, err := schema.ParseGroupVersion(apiResourcesList.GroupVersion)
			if err != nil {
				logger.Error(err, "Failed to parse GroupVersion", "GroupVersion", apiResourcesList.GroupVersion)
				continue
			}
			apiResourcesListGroup = gv.Group
		}

		for _, apiResource := range apiResourcesList.APIResources {
			// Use apiResource.Group if explicitly set (aggregated discovery),
			// otherwise fall back to the list's group (legacy discovery)
			apiResourceGroup := apiResource.Group
			if apiResourceGroup == "" {
				apiResourceGroup = apiResourcesListGroup
			}

			if apiResource.Kind == gk.Kind && apiResourceGroup == gk.Group {
				return true
			}
		}
	}

	return false
}
