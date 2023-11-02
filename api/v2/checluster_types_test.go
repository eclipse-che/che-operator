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

package v2

import (
	"reflect"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/stretchr/testify/assert"
)

func TestIsAccesTokenConfigured(t *testing.T) {
	t.Run("TestIsAccesTokenConfigured when access_token defined", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "access_token",
					},
				}},
		}
		assert.True(t, cheCluster.IsAccessTokenConfigured(), "'access_token' should be activated")
	})
	t.Run("TestIsAccesTokenConfigured when id_token defined", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "id_token",
					},
				}},
		}
		assert.False(t, cheCluster.IsAccessTokenConfigured(), "'access_token' should not be activated")
	})
}

func TestGetIdentityToken(t *testing.T) {
	t.Run("TestGetIdentityToken when access_token defined in config and k8s", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "access_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, "access_token", cheCluster.GetIdentityToken(),
			"'access_token' should be used")
	})

	t.Run("TestGetIdentityToken when id_token defined in config and k8s", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "id_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, "id_token", cheCluster.GetIdentityToken(),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when no defined token in config and k8s", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, "id_token", cheCluster.GetIdentityToken(),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when access_token defined in config and openshift", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "access_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, "access_token", cheCluster.GetIdentityToken(),
			"'access_token' should be used")
	})

	t.Run("TestGetIdentityToken when id_token defined in config and openshift", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{
						IdentityToken: "id_token",
					},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, "id_token", cheCluster.GetIdentityToken(),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when no defined token in config and openshift", func(t *testing.T) {
		cheCluster := &CheCluster{
			Spec: CheClusterSpec{
				Networking: CheClusterSpecNetworking{
					Auth: Auth{},
				}},
		}
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, "access_token", cheCluster.GetIdentityToken(),
			"'access_token' should be used")
	})

}

func TestGetDefaultIdentityToken(t *testing.T) {
	emptyCheCluster := CheCluster{}

	var tests = []struct {
		infrastructure infrastructure.Type
		identityToken  string
	}{
		{infrastructure.OpenShiftv4, "access_token"},
		{infrastructure.Kubernetes, "id_token"},
		{infrastructure.Unsupported, "id_token"},
	}
	for _, test := range tests {
		infrastructure.InitializeForTesting(test.infrastructure)
		if actual := emptyCheCluster.GetIdentityToken(); !reflect.DeepEqual(test.identityToken, actual) {
			t.Errorf("Test Failed. Expected '%s', but got '%s'", test.identityToken, actual)
		}
	}
}
