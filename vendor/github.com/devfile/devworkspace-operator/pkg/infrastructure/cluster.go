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

package infrastructure

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Type specifies what kind of infrastructure we're operating in.
type Type int

const (
	// Unsupported represents an Unsupported cluster version (e.g. OpenShift v3)
	Unsupported Type = iota
	Kubernetes
	OpenShiftv4
)

var (
	// current is the infrastructure that we're currently running on.
	current     Type
	initialized = false
)

// Initialize attempts to determine the type of cluster its currently running on (OpenShift or Kubernetes). This function
// *must* be called before others; otherwise the call will panic.
func Initialize() error {
	var err error
	current, err = detect()
	if err != nil {
		return err
	}
	if current == Unsupported {
		return fmt.Errorf("running on unsupported cluster")
	}
	initialized = true
	return nil
}

// InitializeForTesting is used to mock running on a specific type of cluster (Kubernetes, OpenShift) in testing code.
func InitializeForTesting(currentInfrastructure Type) {
	current = currentInfrastructure
	initialized = true
}

// IsOpenShift returns true if the current cluster is an OpenShift (v4.x) cluster.
func IsOpenShift() bool {
	if !initialized {
		panic("Attempting to determine information about the cluster without initializing first")
	}
	return current == OpenShiftv4
}

func detect() (Type, error) {
	kubeCfg, err := config.GetConfig()
	if err != nil {
		return Unsupported, fmt.Errorf("could not get kube config: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeCfg)
	if err != nil {
		return Unsupported, fmt.Errorf("could not get discovery client: %w", err)
	}
	apiList, err := discoveryClient.ServerGroups()
	if err != nil {
		return Unsupported, fmt.Errorf("could not read API groups: %w", err)
	}
	if findAPIGroup(apiList.Groups, "route.openshift.io") == nil {
		return Kubernetes, nil
	} else {
		if findAPIGroup(apiList.Groups, "config.openshift.io") == nil {
			return Unsupported, nil
		} else {
			return OpenShiftv4, nil
		}
	}
}

func findAPIGroup(source []metav1.APIGroup, apiName string) *metav1.APIGroup {
	for i := 0; i < len(source); i++ {
		if source[i].Name == apiName {
			return &source[i]
		}
	}
	return nil
}

func findAPIResources(source []*metav1.APIResourceList, groupName string) []metav1.APIResource {
	for i := 0; i < len(source); i++ {
		if source[i].GroupVersion == groupName {
			return source[i].APIResources
		}
	}
	return nil
}
