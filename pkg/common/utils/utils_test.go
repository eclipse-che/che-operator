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

package utils

import (
	"reflect"
	"strings"
	"testing"
)

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

func TestRandomString(t *testing.T) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	if actual := RandomString(0); actual != "" {
		t.Errorf("Test Failed. Expected empty string for length 0, but got '%s'", actual)
	}

	if actual := RandomString(-5); actual != "" {
		t.Errorf("Test Failed. Expected empty string for negative length, but got '%s'", actual)
	}

	for _, length := range []int{1, 12, 32, 256} {
		actual := RandomString(length)
		if len(actual) != length {
			t.Errorf("Test Failed. Expected string of length %d, but got length %d", length, len(actual))
		}

		for _, c := range actual {
			if !strings.ContainsRune(charset, c) {
				t.Errorf("Test Failed. Expected only charset characters, but got '%c'", c)
			}
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
