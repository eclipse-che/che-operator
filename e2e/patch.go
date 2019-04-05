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
package main

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	api "k8s.io/apimachinery/pkg/types"
)

func patchCustomResource(path string, value bool) (err error) {

	type PatchSpec struct {
		Operation    string `json:"op"`
		Path  string `json:"path"`
		Value bool `json:"value"`
	}

	fields := make([]PatchSpec, 1)
	fields[0].Operation = "replace"
	fields[0].Path = path
	fields[0].Value = value
	patchBytes, err := json.Marshal(fields)
	if err != nil {
		logrus.Errorf("Failed to marchall fields %s", err)
		return err
	}
	_, err = clientSet.restClient.Patch(api.JSONPatchType).Name(crName).Namespace(namespace).Resource(kind).Body(patchBytes).Do().Get()

	if err != nil {
		logrus.Errorf("Failed to patch CR: %s", err)
		return err
	}

	return nil
}