//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package diffs

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigMap(t *testing.T) {
	type testCase struct {
		srcCm   *corev1.ConfigMap
		dstCm   *corev1.ConfigMap
		diffs   cmp.Options
		isEqual bool
	}

	testCases := []testCase{
		{
			srcCm:   &corev1.ConfigMap{},
			dstCm:   &corev1.ConfigMap{},
			diffs:   ConfigMapAllLabels,
			isEqual: true,
		},
		{
			srcCm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			},
			dstCm:   &corev1.ConfigMap{},
			diffs:   ConfigMapAllLabels,
			isEqual: true,
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("Test case %d", i), func(t *testing.T) {
			assert.Equal(
				t,
				testCase.isEqual,
				cmp.Equal(testCase.srcCm, testCase.dstCm, testCase.diffs),
			)
		})
	}
}
