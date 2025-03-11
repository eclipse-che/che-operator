//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"os"
	"reflect"
	"testing"

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
		{"test.com", "test.com"},
	}
	for _, test := range tests {
		if actual := Whitelist(test.host); !reflect.DeepEqual(test.whitelistedHost, actual) {
			t.Errorf("Test Failed. Expected '%s', but got '%s'", test.whitelistedHost, actual)
		}
	}
}

func TestMergeMaps(t *testing.T) {
	map1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	map2 := map[string]string{
		"key3": "value3",
		"key4": "value4",
	}
	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
	}

	actual := MergeMaps([]map[string]string{nil, map1, nil, map2, make(map[string]string)})
	assert.Equal(t, expected, actual)
}

func TestGetEnvOrDefault(t *testing.T) {
	var tests = []struct {
		envName       string
		envValue      string
		defaultValue  string
		expectedValue string
	}{
		{"env1", "", "default1", "default1"},
		{"env2", "value2", "default2", "value2"},
	}

	_ = os.Setenv("env2", "value2")

	defer func() {
		_ = os.Unsetenv("env1")
		_ = os.Unsetenv("env2")
	}()

	for _, test := range tests {
		assert.Equal(t, test.expectedValue, GetEnvOrDefault(test.envName, test.defaultValue))
	}
}
