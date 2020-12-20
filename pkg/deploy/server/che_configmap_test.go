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
package server

import (
	"github.com/eclipse/che-operator/pkg/deploy"
	"strings"
	"testing"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
)

func createFakeDiscovery() *fakeDiscovery.FakeDiscovery {
	clientSet := fakeclientset.NewSimpleClientset()
	fakeDiscovery, _ := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
	fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "route.openshift.io/v1",
			APIResources: []metav1.APIResource{
				{
					Kind: "Route",
				},
			},
		},
	}
	return fakeDiscovery
}

func TestNewCheConfigMap(t *testing.T) {

	// since all values are retrieved from CR or auto-generated
	// some of them are explicitly set for this test to avoid using fake kube
	// and creating a CR with all spec fields pre-populated
	cr := &orgv1.CheCluster{}
	cr.Spec.Server.CheHost = "myhostname.com"
	cr.Spec.Server.TlsSupport = true
	cr.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(true)
	deployContext := &deploy.DeployContext{
		CheCluster: cr,
		Proxy:      &deploy.Proxy{},
		ClusterAPI: deploy.ClusterAPI{
			DiscoveryClient: createFakeDiscovery(),
		},
	}
	cheEnv, _ := GetCheConfigMapData(deployContext)
	testCm, _ := deploy.GetSpecConfigMap(deployContext, CheConfigMapName, cheEnv)
	identityProvider := testCm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"]
	util.DetectOpenShift(deployContext.ClusterAPI.DiscoveryClient)
	protocol := strings.Split(testCm.Data["CHE_API"], "://")[0]
	expectedIdentityProvider := "openshift-v3"
	if util.IsOpenshift4() {
		expectedIdentityProvider = "openshift-v4"
	}
	if identityProvider != expectedIdentityProvider {
		t.Errorf("Test failed. Expecting identity provider to be '%s' while got '%s'", expectedIdentityProvider, identityProvider)
	}
	if protocol != "https" {
		t.Errorf("Test failed. Expecting 'https' protocol, got '%s'", protocol)
	}
}

func TestConfigMapOverride(t *testing.T) {
	cr := &orgv1.CheCluster{}
	cr.Spec.Server.CheHost = "myhostname.com"
	cr.Spec.Server.TlsSupport = true
	cr.Spec.Server.CustomCheProperties = map[string]string{
		"CHE_WORKSPACE_NO_PROXY": "myproxy.myhostname.com",
	}
	cr.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(true)
	deployContext := &deploy.DeployContext{
		CheCluster: cr,
		Proxy:      &deploy.Proxy{},
		ClusterAPI: deploy.ClusterAPI{
			DiscoveryClient: createFakeDiscovery(),
		},
	}
	cheEnv, _ := GetCheConfigMapData(deployContext)
	testCm, _ := deploy.GetSpecConfigMap(deployContext, CheConfigMapName, cheEnv)
	if testCm.Data["CHE_WORKSPACE_NO_PROXY"] != "myproxy.myhostname.com" {
		t.Errorf("Test failed. Expected myproxy.myhostname.com but was %s", testCm.Data["CHE_WORKSPACE_NO_PROXY"])
	}

}
