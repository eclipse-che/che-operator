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
	"strings"
	"testing"

	"github.com/eclipse/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
)

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
		ClusterAPI: deploy.ClusterAPI{},
	}
	cheEnv, _ := GetCheConfigMapData(deployContext)
	testCm, _ := deploy.GetSpecConfigMap(deployContext, CheConfigMapName, cheEnv, CheConfigMapName)
	identityProvider := testCm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"]
	_, isOpenshiftv4, _ := util.DetectOpenShift()
	protocol := strings.Split(testCm.Data["CHE_API"], "://")[0]
	expectedIdentityProvider := "openshift-v3"
	if isOpenshiftv4 {
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
		ClusterAPI: deploy.ClusterAPI{},
	}
	cheEnv, _ := GetCheConfigMapData(deployContext)
	testCm, _ := deploy.GetSpecConfigMap(deployContext, CheConfigMapName, cheEnv, CheConfigMapName)
	if testCm.Data["CHE_WORKSPACE_NO_PROXY"] != "myproxy.myhostname.com" {
		t.Errorf("Test failed. Expected myproxy.myhostname.com but was %s", testCm.Data["CHE_WORKSPACE_NO_PROXY"])
	}

}
