//
// Copyright (c) 2012-2018 Red Hat, Inc.
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
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"math/rand"
	"os"
)

// GetEnvValue looks for env variables in Operator pod to configure Code Ready deployments
// with things like db users, passwords and deployment options in general. Envs are set in
// a ConfigMap at deploy/config.yaml. Find more details on deployment options in README.md
func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func GetEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	if value == "true" {
		return true
	}
	return false
}

func GeneratePasswd(stringLength int) (passwd string) {
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := stringLength
	buf := make([]rune, length)
	for i := range buf {
		buf[i] = chars[rand.Intn(len(chars))]
	}
	passwd = string(buf)
	return passwd
}

func GetInfra() (infra string) {
	// set infra via env var, for instance for testing purposes
	infraEnv := os.Getenv("INFRA")
	if len(infraEnv) != 0 {
		infra = infraEnv
		return infra
	} else {
		kubeClient := k8sclient.GetKubeClient()
		serverGroups, _ := kubeClient.Discovery().ServerGroups()
		apiGroups := serverGroups.Groups

		for i := range apiGroups {
			name := apiGroups[i].Name
			if name == "route.openshift.io" {
				infra = "openshift"
			}
		}
		if infra == "" {
			infra = "kubernetes"
		}
		return infra
	}
}

func GetNamespace() (currentNamespace string) {

	namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		logrus.Warnf("Failed to find mounted file with namespace name %s. Using default namespace eclipse-che", err)
		namespace = []byte("eclipse-che")
	}
	currentNamespace = string(namespace)
	return currentNamespace
}
