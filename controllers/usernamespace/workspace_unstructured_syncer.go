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

package usernamespace

import (
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
	"strings"
)

const (
	PROJECT_REQUESTING_USER = "${PROJECT_REQUESTING_USER}"
	PROJECT_NAME            = "${PROJECT_NAME}"
)

type unstructuredSyncer struct {
	WorkspaceSyncObject

	srcObj client.Object
	dstObj client.Object
	hash   string
}

func newUnstructuredSyncer(
	raw []byte,
	user string,
	project string) (*unstructuredSyncer, error) {

	hash := utils.ComputeHash256(raw)

	objAsString := string(raw)
	objAsString = strings.ReplaceAll(objAsString, PROJECT_REQUESTING_USER, user)
	objAsString = strings.ReplaceAll(objAsString, PROJECT_NAME, project)

	srcObj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(objAsString), srcObj); err != nil {
		return nil, err
	}

	dstObj := srcObj.DeepCopyObject()

	return &unstructuredSyncer{
		srcObj: srcObj,
		dstObj: dstObj.(client.Object),
		hash:   hash,
	}, nil
}

func (p *unstructuredSyncer) getSrcObject() client.Object {
	return p.srcObj
}

func (p *unstructuredSyncer) getSrcObjectGKV() schema.GroupVersionKind {
	return p.srcObj.GetObjectKind().GroupVersionKind()
}

func (p *unstructuredSyncer) newDstObject() client.Object {
	return p.dstObj.DeepCopyObject().(client.Object)
}

func (p *unstructuredSyncer) isExistedObjChanged(dstObj client.Object, existedDstObj client.Object) bool {
	if dstObj.GetLabels() != nil {
		for key, value := range dstObj.GetLabels() {
			if existedDstObj.GetLabels()[key] != value {
				return true
			}
		}
	}

	if dstObj.GetAnnotations() != nil {
		for key, value := range dstObj.GetAnnotations() {
			if existedDstObj.GetAnnotations()[key] != value {
				return true
			}
		}
	}

	return cmp.Diff(
		dstObj,
		existedDstObj,
		cmp.Options{
			cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta"),
		}) != ""
}

func (p *unstructuredSyncer) getSrcObjectVersion() string {
	return p.hash
}

func (p *unstructuredSyncer) hasROSpec() bool {
	return false
}
