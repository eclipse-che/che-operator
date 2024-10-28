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
	"strings"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"

	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// Supported templates parameters
	PROJECT_USER = "${PROJECT_USER}"
	PROJECT_NAME = "${PROJECT_NAME}"
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
	objAsString = strings.ReplaceAll(objAsString, PROJECT_USER, user)
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

func (p *unstructuredSyncer) getGKV() schema.GroupVersionKind {
	return p.srcObj.GetObjectKind().GroupVersionKind()
}

func (p *unstructuredSyncer) newDstObject() client.Object {
	dstObj := p.dstObj.DeepCopyObject().(client.Object)

	switch dstObj.GetObjectKind().GroupVersionKind().String() {
	case v1ConfigMapGKV.String():
		dstObj.SetLabels(utils.MergeMaps([]map[string]string{
			dstObj.GetLabels(),
			{
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
				dwconstants.DevWorkspaceMountLabel:          "true",
			}}),
		)
		break
	case v1SecretGKV.String():
		dstObj.SetLabels(utils.MergeMaps([]map[string]string{
			dstObj.GetLabels(),
			{
				dwconstants.DevWorkspaceWatchSecretLabel: "true",
				dwconstants.DevWorkspaceMountLabel:       "true",
			}}),
		)
		break
	}

	return dstObj
}

func (p *unstructuredSyncer) getSrcObjectVersion() string {
	return p.hash
}

func (p *unstructuredSyncer) hasROSpec() bool {
	switch p.dstObj.GetObjectKind().GroupVersionKind().String() {
	case v1PvcGKV.String():
		return true
	}
	return false
}
