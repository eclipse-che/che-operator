//
// Copyright (c) 2012-2021 Red Hat, Inc.
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
