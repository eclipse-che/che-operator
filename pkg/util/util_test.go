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
package util

import (
	"reflect"
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
)

func TestGeneratePasswd(t *testing.T) {
	chars := 12
	passwd := GeneratePasswd(chars)
	expectedCharsNumber := 12

	if !reflect.DeepEqual(len(passwd), expectedCharsNumber) {
		t.Errorf("Test failed. Expected %v chars, got %v chars", expectedCharsNumber, len(passwd))
	}

	passwd1 := GeneratePasswd(12)
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

func TestIsCheMultiUser(t *testing.T) {
	var tests = []struct {
		checluster             *orgv1.CheCluster
		expectedIsCheMultiUser bool
	}{
		{&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{},
			},
		}, true},
		{&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					CustomCheProperties: map[string]string{},
				},
			},
		}, true},
		{&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					CustomCheProperties: map[string]string{
						"CHE_MULTIUSER": "true",
					},
				},
			},
		}, true},
		{&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					CustomCheProperties: map[string]string{
						"CHE_MULTIUSER": "false",
					},
				},
			},
		}, false},
	}

	for _, test := range tests {
		actualIsCheMultiUser := IsCheMultiUser(test.checluster)
		if actualIsCheMultiUser != test.expectedIsCheMultiUser {
			t.Errorf("Test Failed. Expected '%t', but got '%t'", test.expectedIsCheMultiUser, actualIsCheMultiUser)
		}
	}
}
