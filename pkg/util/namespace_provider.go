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
	"errors"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
)

const (
	namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

var operator_namespace string

func readNamespace() string {
	nsBytes, err := ioutil.ReadFile(namespaceFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ""
		}
		logrus.Fatal("Failed to get operator namespace", err)
	}
	return string(nsBytes)
}

// GetCheOperatorNamespace returns namespace for current Eclipse Che operator.
func GetCheOperatorNamespace() string {
	if operator_namespace == "" && !IsTestMode() {
		operator_namespace = readNamespace()
	}
	return operator_namespace
}
