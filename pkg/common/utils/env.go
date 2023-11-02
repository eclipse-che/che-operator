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

package utils

import (
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func GetEnvByName(name string, envs []corev1.EnvVar) string {
	for _, env := range envs {
		if env.Name == name {
			return env.Value
		}
	}
	return ""
}

func IndexEnv(name string, envs []corev1.EnvVar) int {
	for i, env := range envs {
		if env.Name == name {
			return i
		}
	}

	return -1
}

func GetGetArchitectureDependentEnvsByRegExp(regExp string) []corev1.EnvVar {
	return doGetEnvsByRegExp(regExp, true)
}

func GetEnvsByRegExp(regExp string) []corev1.EnvVar {
	return doGetEnvsByRegExp(regExp, false)
}

func doGetEnvsByRegExp(regExp string, isArchitectureDependentEnvNameNeeded bool) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		envName := pair[0]
		rxp := regexp.MustCompile(regExp)
		if rxp.MatchString(envName) {
			if isArchitectureDependentEnvNameNeeded {
				envName = GetArchitectureDependentEnvName(envName)
			}
			env = append(env, corev1.EnvVar{Name: envName, Value: pair[1]})
		}
	}

	sort.Slice(env, func(i, j int) bool {
		return strings.Compare(env[i].Name, env[j].Name) < 0
	})

	return env
}

// GetArchitectureDependentEnvName returns environment variable name dependending on architecture
// by adding "_<ARCHITECTURE>" suffix. If variable is not set then the default will be return.
func GetArchitectureDependentEnvName(name string) string {
	archName := name + "_" + runtime.GOARCH
	if _, ok := os.LookupEnv(archName); ok {
		return archName
	}

	return name
}
