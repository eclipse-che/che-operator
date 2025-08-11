//
// Copyright (c) 2019-2024 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package workspace_config

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	// Supported templates parameters
	PROJECT_ADMIN_USER = "${PROJECT_ADMIN_USER}"
	PROJECT_NAME       = "${PROJECT_NAME}"
)

var (
	v1ConfigMapGKV = corev1.SchemeGroupVersion.WithKind("ConfigMap")
	v1SecretGKV    = corev1.SchemeGroupVersion.WithKind("Secret")
	v1PvcGKV       = corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")
)

func createObject2SyncFromRaw(
	raw []byte,
	userName string,
	namespaceName string) (Object2Sync, error) {

	hash := utils.ComputeHash256(raw)

	objAsString := string(raw)
	objAsString = strings.ReplaceAll(objAsString, PROJECT_ADMIN_USER, userName)
	objAsString = strings.ReplaceAll(objAsString, PROJECT_NAME, namespaceName)

	srcObj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(objAsString), srcObj); err != nil {
		return nil, err
	}

	gkv := srcObj.GetObjectKind().GroupVersionKind()
	switch gkv {
	case v1ConfigMapGKV:
		cm := &corev1.ConfigMap{}
		if err := yaml.Unmarshal([]byte(objAsString), cm); err != nil {
			return nil, err
		}

		return &configMap2Sync{
			cm:      cm,
			version: hash,
		}, nil

	case v1SecretGKV:
		secret := &corev1.Secret{}
		if err := yaml.Unmarshal([]byte(objAsString), secret); err != nil {
			return nil, err
		}

		return &secret2Sync{
			secret:  secret,
			version: hash,
		}, nil

	case v1PvcGKV:
		pvc := &corev1.PersistentVolumeClaim{}
		if err := yaml.Unmarshal([]byte(objAsString), pvc); err != nil {
			return nil, err
		}

		return &pvc2Sync{
			pvc:     pvc,
			version: hash,
		}, nil
	}

	return &unstructured2Sync{
		srcObj:  srcObj,
		dstObj:  srcObj,
		version: hash,
	}, nil
}

func createObject2SyncFromRuntimeObject(obj runtime.Object) Object2Sync {
	gkv := obj.GetObjectKind().GroupVersionKind()
	switch gkv {
	case v1ConfigMapGKV:
		cm := obj.(*corev1.ConfigMap)
		return &configMap2Sync{cm: cm}

	case v1SecretGKV:
		secret := obj.(*corev1.Secret)
		return &secret2Sync{secret: secret}

	case v1PvcGKV:
		pvc := obj.(*corev1.PersistentVolumeClaim)
		return &pvc2Sync{pvc: pvc}
	}

	return nil
}
