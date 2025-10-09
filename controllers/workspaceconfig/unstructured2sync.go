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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type unstructured2Sync struct {
	Object2Sync

	srcObj  client.Object
	version string
}

func (p *unstructured2Sync) getSrcObject() client.Object {
	return p.srcObj
}

func (p *unstructured2Sync) getGKV() schema.GroupVersionKind {
	return p.srcObj.GetObjectKind().GroupVersionKind()
}

func (p *unstructured2Sync) newDstObject() client.Object {
	dstObj := p.srcObj.DeepCopyObject().(client.Object)
	return dstObj
}

func (p *unstructured2Sync) getSrcObjectVersion() string {
	return p.version
}

func (p *unstructured2Sync) hasROSpec() bool {
	return false
}

func (p *unstructured2Sync) defaultRetention() bool {
	return false
}
