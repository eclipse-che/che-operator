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
package util

import (
	"github.com/sirupsen/logrus"
	"os"
	"reflect"
	"testing"
)

const (
	proxyHost                               = "https://myproxy.com"
	proxyPort                               = "1234"
	nonProxyHosts                           = "localhost|myhost.com"
	proxyUser                               = "user"
	proxyPassword                           = "password"
	expectedProxyURLWithUsernamePassword    = "https://user:password@myproxy.com:1234"
	expectedProxyURLWithoutUsernamePassword = "https://myproxy.com:1234"
	expectedNoProxy                         = "localhost,myhost.com"
)

func TestGenerateProxyEnvs(t *testing.T) {

	proxyUrl, noProxy, _ := GenerateProxyEnvs(proxyHost, proxyPort, nonProxyHosts, proxyUser, proxyPassword, "", "")

	if !reflect.DeepEqual(proxyUrl, expectedProxyURLWithUsernamePassword) {
		t.Errorf("Test failed. Expected %s but got %s", expectedProxyURLWithUsernamePassword, proxyUrl)
	}

	if !reflect.DeepEqual(noProxy, expectedNoProxy) {
		t.Errorf("Test failed. Expected %s but got %s", expectedNoProxy, noProxy)

	}

	proxyUrl, _, _ = GenerateProxyEnvs(proxyHost, proxyPort, nonProxyHosts, "", proxyPassword, "", "")
	if !reflect.DeepEqual(proxyUrl, expectedProxyURLWithoutUsernamePassword) {
		t.Errorf("Test failed. Expected %s but got %s", expectedProxyURLWithoutUsernamePassword, proxyUrl)
	}

}

func TestGenerateProxyJavaOpts(t *testing.T) {
	if err := os.Setenv("KUBERNETES_SERVICE_HOST", "172.30.0.1"); err != nil {
		logrus.Errorf("Failed to set env %s", err)
	}

	javaOpts, _ := GenerateProxyJavaOpts(proxyHost, proxyPort, nonProxyHosts, proxyUser, proxyPassword, "", "")
	expectedJavaOpts := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
		"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='localhost|myhost.com' -Dhttp.proxyUser=user " +
		"-Dhttp.proxyPassword=password -Dhttps.proxyUser=user -Dhttps.proxyPassword=password"
	if !reflect.DeepEqual(javaOpts,expectedJavaOpts) {
		t.Errorf("Test failed. Expected '%s' but got '%s'", expectedJavaOpts, javaOpts)

	}

	javaOpts, _ = GenerateProxyJavaOpts(proxyHost, proxyPort, nonProxyHosts, "", proxyPassword, "", "")
	expectedJavaOptsWithoutUsernamePassword := " -Dhttp.proxyHost=myproxy.com -Dhttp.proxyPort=1234 -Dhttps.proxyHost=myproxy.com " +
		"-Dhttps.proxyPort=1234 -Dhttp.nonProxyHosts='localhost|myhost.com'"

	if !reflect.DeepEqual(javaOpts ,expectedJavaOptsWithoutUsernamePassword) {
		t.Errorf("Test failed. Expected '%s' but got '%s'", expectedJavaOptsWithoutUsernamePassword, javaOpts)

	}
}

func TestGeneratePasswd(t *testing.T) {
	chars := 12
	passwd := GeneratePasswd(chars)
	expectedCharsNumber := 12

	if !reflect.DeepEqual(len(passwd), expectedCharsNumber) {
		t.Errorf("Test failed. Expected %v chars, got %v chars", expectedCharsNumber, len(passwd))
	}

	passwd1 := GeneratePasswd(12)
	if reflect.DeepEqual (passwd, passwd1) {
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
