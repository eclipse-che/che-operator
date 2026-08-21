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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestIsAgentSandboxEnabled(t *testing.T) {
	clientSet := fakeclientset.NewClientset()
	discoveryClient := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)

	if IsAgentSandboxEnabled(discoveryClient) {
		t.Error("expected agent-sandbox not detected when resource absent")
	}

	discoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "agents.x-k8s.io/v1beta1",
			APIResources: []metav1.APIResource{{Name: AgentSandboxResources}},
		},
	}
	if !IsAgentSandboxEnabled(discoveryClient) {
		t.Error("expected agent-sandbox detected when resource present")
	}
}
