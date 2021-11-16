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
package devworkspace

import (
	"io/ioutil"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type CachedObjFile struct {
	data    []byte
	hash256 string
}

var (
	// cachedObjects
	cachedObjFiles = make(map[string]*CachedObjFile)
)

// readK8SObject reads DWO related object from file system and cache value to avoid read later
// returned object already has provisioned annotation with object hash
func readK8SUnstructured(yamlFile string, into *unstructured.Unstructured) error {
	hash, err := readObj(yamlFile, into)
	if err != nil {
		return err
	}

	if into.GetAnnotations() == nil {
		annotations := map[string]string{}
		into.SetAnnotations(annotations)
	}

	into.GetAnnotations()[deploy.CheEclipseOrgHash256] = hash
	return nil
}

// readK8SObject reads DWO related object from file system and cache value to avoid read later
// returned object already has provisioned annotation with object hash
func readK8SObject(yamlFile string, into metav1.Object) error {
	hash, err := readObj(yamlFile, into)
	if err != nil {
		return err
	}

	if into.GetAnnotations() == nil {
		annotations := map[string]string{}
		into.SetAnnotations(annotations)
	}

	into.GetAnnotations()[deploy.CheEclipseOrgHash256] = hash
	return nil
}

func readObj(yamlFile string, into interface{}) (string, error) {
	cachedFile, exists := cachedObjFiles[yamlFile]
	if !exists {
		data, err := ioutil.ReadFile(yamlFile)
		if err != nil {
			return "", err
		}

		cachedFile = &CachedObjFile{
			data,
			util.ComputeHash256(data),
		}
		cachedObjFiles[yamlFile] = cachedFile
	}

	err := yaml.Unmarshal(cachedFile.data, into)
	if err != nil {
		return "", err
	}
	return cachedFile.hash256, nil
}
