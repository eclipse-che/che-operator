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
package deploy

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"strings"
	"testing"
)

func TestNewCheConfigMap(t *testing.T) {

	// since all values are retrieved from CR or auto-generated
	// some of them are explicitly set for this test to avoid using fake kube
	// and creating a CR with all spec fields pre-populated
	cr := &orgv1.CheCluster{}
	cr.Spec.Server.CheHost = "myhostname.com"
	cr.Spec.Server.TlsSupport = true
	cr.Spec.Auth.OpenShiftOauth = true
	cheEnv := GetConfigMapData(cr)
	testCm := NewCheConfigMap(cr, cheEnv)
	identityProvider := testCm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"]
	protocol := strings.Split(testCm.Data["CHE_INFRA_KUBERNETES_BOOTSTRAPPER_BINARY__URL"], "://")[0]
	if identityProvider != "openshift-v3" {
		t.Errorf("Test failed. Expecting identity provider to be 'openshift-v3' while got '%s'", identityProvider)
	}
	if protocol != "https" {
		t.Errorf("Test failed. Expecting 'https' protocol, got '%s'", protocol)
	}
}
