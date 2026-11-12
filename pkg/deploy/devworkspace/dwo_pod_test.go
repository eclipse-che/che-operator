//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIsDevWorkspaceOperatorPod(t *testing.T) {
	dwoPod := test.NewDWOPod("devworkspace-controller", "project-clone:dwo")

	otherServiceAccount := dwoPod.DeepCopy()
	otherServiceAccount.Spec.ServiceAccountName = "default"

	webhookServerPod := dwoPod.DeepCopy()
	webhookServerPod.Labels["app.kubernetes.io/name"] = "devworkspace-webhook-server"

	type testCase struct {
		name     string
		obj      client.Object
		expected bool
	}

	testCases := []testCase{
		{name: "DevWorkspace Operator controller pod", obj: dwoPod, expected: true},
		{name: "Pod with another service account", obj: otherServiceAccount, expected: false},
		{name: "DevWorkspace Operator webhook server pod", obj: webhookServerPod, expected: false},
		{
			name:     "ConfigMap with DevWorkspace Operator controller labels",
			obj:      &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Labels: dwoPod.Labels}},
			expected: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, IsDevWorkspaceOperatorPod(testCase.obj))
		})
	}
}
