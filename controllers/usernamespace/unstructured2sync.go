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
	PROJECT_ADMIN_USER = "${PROJECT_ADMIN_USER}"
	PROJECT_NAME       = "${PROJECT_NAME}"
)

type unstructured2Sync struct {
	Object2Sync

	srcObj client.Object
	dstObj client.Object
	hash   string
}

func newUnstructured2Sync(
	raw []byte,
	userName string,
	namespaceName string) (*unstructured2Sync, error) {

	hash := utils.ComputeHash256(raw)

	objAsString := string(raw)
	objAsString = strings.ReplaceAll(objAsString, PROJECT_ADMIN_USER, userName)
	objAsString = strings.ReplaceAll(objAsString, PROJECT_NAME, namespaceName)

	srcObj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(objAsString), srcObj); err != nil {
		return nil, err
	}

	dstObj := srcObj.DeepCopyObject()

	return &unstructured2Sync{
		srcObj: srcObj,
		dstObj: dstObj.(client.Object),
		hash:   hash,
	}, nil
}

func (p *unstructured2Sync) getSrcObject() client.Object {
	return p.srcObj
}

func (p *unstructured2Sync) getGKV() schema.GroupVersionKind {
	return p.srcObj.GetObjectKind().GroupVersionKind()
}

func (p *unstructured2Sync) newDstObject() client.Object {
	dstObj := p.dstObj.DeepCopyObject().(client.Object)

	switch dstObj.GetObjectKind().GroupVersionKind() {
	case v1ConfigMapGKV:
		dstObj.SetLabels(utils.MergeMaps([]map[string]string{
			dstObj.GetLabels(),
			{
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
				dwconstants.DevWorkspaceMountLabel:          "true",
			}}),
		)
		break
	case v1SecretGKV:
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

func (p *unstructured2Sync) getSrcObjectVersion() string {
	return p.hash
}

func (p *unstructured2Sync) hasROSpec() bool {
	return p.dstObj.GetObjectKind().GroupVersionKind() == v1PvcGKV
}

func (p *unstructured2Sync) isDiff(obj client.Object) bool {
	return isLabelsOrAnnotationsDiff(p.srcObj, obj) || isUnstructuredDiff(p.srcObj, obj)
}
