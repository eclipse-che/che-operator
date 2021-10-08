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

package devworkspace

import (
	"context"
	"time"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	chev2alpha1 "github.com/eclipse-che/che-operator/api/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type DevWorkspaceState int

const (
	APINotPresentState DevWorkspaceState = 0
	DisabledState      DevWorkspaceState = 1
	EnabledState       DevWorkspaceState = 2
)

// ShouldDevWorkspacesBeEnabled evaluates if DevWorkspace mode should be enabled
// which we only do, if there is the controller.devfile.io resource group in the cluster
// and DevWorkspaces are enabled at least on one CheCluster
func ShouldDevWorkspacesBeEnabled(mgr manager.Manager) (bool, error) {
	dwEnabled, err := doesCheClusterWithDevWorkspaceEnabledExist(mgr)
	if err != nil {
		return false, err
	}

	if !dwEnabled {
		return false, nil
	}

	// we assume that if the group is there, then we have all the expected CRs there, too.
	dwApiExists, err := findApiGroup(mgr, "controller.devfile.io")
	if err != nil {
		return false, err
	}

	if !dwApiExists {
		log.Info("WARN: there is a CheCluster with DevWorkspace enabled but devworkspace api group 'controller.devfile.io' is not available." +
			"DevWorkspace mode is not activating assuming that Che Operator will install it and initiate reboot")
		return false, nil
	}

	return true, nil
}

func NotifyWhenDevWorkspaceEnabled(mgr manager.Manager, stop <-chan struct{}, callback func()) {
	for {
		select {
		case <-stop:
			return
		case <-time.After(time.Duration(60) * time.Second):
			// don't spam the log every time we check. The first time was enough...
			shouldDevWorkspacesBeEnabled, err := ShouldDevWorkspacesBeEnabled(mgr)
			if err != nil {
				log.Error(err, "Failed to check if there is any CheCluster with DevWorkspaces enabled. DevWorkspace mode is not activated")
			}
			if shouldDevWorkspacesBeEnabled {
				callback()
			}
		}
	}
}

func GetDevWorkspaceState(scheme *runtime.Scheme, cr *chev2alpha1.CheCluster) DevWorkspaceState {
	if !scheme.IsGroupRegistered("controller.devfile.io") {
		return APINotPresentState
	}

	if !cr.Spec.IsEnabled() {
		return DisabledState
	}

	return EnabledState
}

var nonCachedClient *client.Client

func doesCheClusterWithDevWorkspaceEnabledExist(mgr manager.Manager) (bool, error) {
	if nonCachedClient == nil {
		c, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
		})
		if err != nil {
			return false, err
		}
		nonCachedClient = &c
	}

	cheClusters := &orgv1.CheClusterList{}
	err := (*nonCachedClient).List(context.TODO(), cheClusters, &client.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, cheCluster := range cheClusters.Items {
		if cheCluster.Spec.DevWorkspace.Enable {
			return true, nil
		}
	}
	return false, nil
}

func findApiGroup(mgr manager.Manager, apiGroup string) (bool, error) {
	cl, err := discovery.NewDiscoveryClientForConfig(mgr.GetConfig())
	if err != nil {
		return false, err
	}

	groups, err := cl.ServerGroups()
	if err != nil {
		return false, err
	}

	supported := false
	for _, g := range groups.Groups {
		if g.Name == apiGroup {
			supported = true
			break
		}
	}
	return supported, nil
}
