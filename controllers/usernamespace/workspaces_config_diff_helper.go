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
	"encoding/json"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// isDiff checks if the given objects are different.
// The rules are following:
//   - if labels of the source object are absent in the destination object,
//     then the objects considered different
//   - if annotations of the source object are absent in the destination object,
//     then the objects considered different
//   - if the rest fields of the objects are different,
//     then the objects considered different
func isDiff(src client.Object, dst client.Object) bool {
	_, isSrcUnstructured := src.(*unstructured.Unstructured)
	_, isDstUnstructured := dst.(*unstructured.Unstructured)

	if !isSrcUnstructured && !isDstUnstructured {
		return isLabelsOrAnnotationsDiff(src, dst) ||
			cmp.Diff(
				src,
				dst,
				cmp.Options{
					cmpopts.IgnoreTypes(metav1.ObjectMeta{}),
					cmpopts.IgnoreTypes(metav1.TypeMeta{}),
				}) != ""
	}

	return isUnstructuredDiff(src, dst)
}

func isUnstructuredDiff(src client.Object, dst client.Object) bool {
	srcUnstructured := toUnstructured(src)
	delete(srcUnstructured.Object, "metadata")
	delete(srcUnstructured.Object, "status")

	dstUnstructured := toUnstructured(dst)
	delete(dstUnstructured.Object, "metadata")
	delete(dstUnstructured.Object, "status")

	return cmp.Diff(srcUnstructured, dstUnstructured) != ""
}

func isLabelsOrAnnotationsDiff(src client.Object, dst client.Object) bool {
	if src.GetLabels() != nil {
		for key, value := range src.GetLabels() {
			if dst.GetLabels()[key] != value {
				return true
			}
		}
	}

	if src.GetAnnotations() != nil {
		for key, value := range src.GetAnnotations() {
			if dst.GetAnnotations()[key] != value {
				return true
			}
		}
	}

	return false
}

func toUnstructured(src client.Object) *unstructured.Unstructured {
	data, err := json.Marshal(src)
	if err != nil {
		logger.Error(err, "Failed to marshal object")
		return nil
	}

	unstructuredObj := &unstructured.Unstructured{}
	err = unstructuredObj.UnmarshalJSON(data)
	if err != nil {
		logger.Error(err, "Failed to unmarshal object")
		return nil
	}

	return unstructuredObj
}
