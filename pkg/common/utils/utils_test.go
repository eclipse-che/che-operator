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
package utils

import (
	"reflect"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
)

func TestGeneratePasswd(t *testing.T) {
	chars := 12
	passwd := GeneratePassword(chars)
	expectedCharsNumber := 12

	if !reflect.DeepEqual(len(passwd), expectedCharsNumber) {
		t.Errorf("Test failed. Expected %v chars, got %v chars", expectedCharsNumber, len(passwd))
	}

	passwd1 := GeneratePassword(12)
	if reflect.DeepEqual(passwd, passwd1) {
		t.Errorf("Test failed. Passwords are identical, %s: %s", passwd, passwd1)
	}
}

func TestGetValue(t *testing.T) {
	key := "myvalue"
	defaultValue := "myDefaultValue"
	var1 := GetValue(key, defaultValue)
	var2 := GetValue("", defaultValue)

	if !reflect.DeepEqual(var1, key) {
		t.Errorf("Test failed. Expected '%s', but got '%s'", key, var1)
	}

	if !reflect.DeepEqual(var2, defaultValue) {
		t.Errorf("Test failed. Expected '%s', but got '%s'", var2, defaultValue)
	}
}

func TestGetImageNameAndTag(t *testing.T) {
	var tests = []struct {
		input        string
		expectedName string
		expectedTag  string
	}{
		{"quay.io/test/test:tag", "quay.io/test/test", "tag"},
		{"quay.io/test/test@sha256:abcdef", "quay.io/test/test", "sha256:abcdef"},
		{"quay.io/test/test", "quay.io/test/test", "latest"},
		{"localhost:5000/test", "localhost:5000/test", "latest"},
	}
	for _, test := range tests {
		if actualName, actualTag := GetImageNameAndTag(test.input); actualName != test.expectedName || actualTag != test.expectedTag {
			t.Errorf("Test Failed. Expected '%s' and '%s', but got '%s' and '%s'", test.expectedName, test.expectedTag, actualName, actualTag)
		}
	}
}

func TestWhitelist(t *testing.T) {
	var tests = []struct {
		host            string
		whitelistedHost string
	}{
		{"che.qwruwqlrj.com", ".qwruwqlrj.com"},
		{"one.two.three.four", ".two.three.four"},
		{"abraCadabra-KvakaZybra", "abraCadabra-KvakaZybra"},
		{".", "."},
		{"", ""},
	}
	for _, test := range tests {
		if actual := Whitelist(test.host); !reflect.DeepEqual(test.whitelistedHost, actual) {
			t.Errorf("Test Failed. Expected '%s', but got '%s'", test.whitelistedHost, actual)
		}
	}
}

func TestGetIdentityToken(t *testing.T) {
	t.Run("TestGetIdentityToken when access_token defined in config and k8s", func(t *testing.T) {
		ctx := test.GetDeployContext(
			&chev2.CheCluster{
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityToken: "access_token",
						},
					}},
			}, nil)
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, constants.AccessToken, GetIdentityToken(ctx.CheCluster),
			"'access_token' should be used")
	})

	t.Run("TestGetIdentityToken when id_token defined in config and k8s", func(t *testing.T) {
		ctx := test.GetDeployContext(
			&chev2.CheCluster{
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityToken: "id_token",
						},
					}},
			}, nil)
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, constants.IDToken, GetIdentityToken(ctx.CheCluster),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when no defined token in config and k8s", func(t *testing.T) {
		ctx := test.GetDeployContext(
			&chev2.CheCluster{
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{},
					}},
			}, nil)
		infrastructure.InitializeForTesting(infrastructure.Kubernetes)

		assert.Equal(t, constants.IDToken, GetIdentityToken(ctx.CheCluster),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when access_token defined in config and openshift", func(t *testing.T) {
		ctx := test.GetDeployContext(
			&chev2.CheCluster{
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityToken: "access_token",
						},
					}},
			}, nil)
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, constants.AccessToken, GetIdentityToken(ctx.CheCluster),
			"'access_token' should be used")
	})

	t.Run("TestGetIdentityToken when id_token defined in config and openshift", func(t *testing.T) {
		ctx := test.GetDeployContext(
			&chev2.CheCluster{
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityToken: "id_token",
						},
					}},
			}, nil)
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, constants.IDToken, GetIdentityToken(ctx.CheCluster),
			"'id_token' should be used")
	})

	t.Run("TestGetIdentityToken when no defined token in config and openshift", func(t *testing.T) {
		ctx := test.GetDeployContext(
			&chev2.CheCluster{
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{},
					}},
			}, nil)
		infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

		assert.Equal(t, constants.AccessToken, GetIdentityToken(ctx.CheCluster),
			"'access_token' should be used")
	})

}

func TestGetDefaultIdentityToken(t *testing.T) {
	var tests = []struct {
		infrastructure infrastructure.Type
		identityToken  string
	}{
		{infrastructure.OpenShiftv4, constants.AccessToken},
		{infrastructure.Kubernetes, constants.IDToken},
		{infrastructure.Unsupported, constants.IDToken},
	}
	for _, test := range tests {
		infrastructure.InitializeForTesting(test.infrastructure)
		if actual := GetDefaultIdentityToken(); !reflect.DeepEqual(test.identityToken, actual) {
			t.Errorf("Test Failed. Expected '%s', but got '%s'", test.identityToken, actual)
		}
	}
}
