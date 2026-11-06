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

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetProjectCloneImage(t *testing.T) {
	type testCase struct {
		name                  string
		projectCloneContainer *chev2.Container
		initObjects           []client.Object
		expectedImage         string
	}

	newDWOPod := func(namespace string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "devworkspace-controller",
				Namespace: namespace,
				Labels: map[string]string{
					constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
					constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "devworkspace-controller",
						Env:  []corev1.EnvVar{{Name: "RELATED_IMAGE_project_clone", Value: "project-clone:dwo"}},
					},
				},
			},
		}
	}

	testCases := []testCase{
		{
			name:          "Image from DevWorkspace Operator pod",
			initObjects:   []client.Object{newDWOPod("devworkspace-controller")},
			expectedImage: "project-clone:dwo",
		},
		{
			name:                  "Image from CheCluster",
			projectCloneContainer: &chev2.Container{Image: "project-clone:che"},
			initObjects:           []client.Object{newDWOPod("devworkspace-controller")},
			expectedImage:         "project-clone:che",
		},
		{
			name:          "No DevWorkspace Operator pod",
			expectedImage: "",
		},
		{
			name:          "DevWorkspace Operator pod in another namespace",
			initObjects:   []client.Object{newDWOPod("other-namespace")},
			expectedImage: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cheCtx := test.NewCtxBuilder().WithObjects(testCase.initObjects...).Build()
			cheCtx.CheCluster.Spec.DevEnvironments.ProjectCloneContainer = testCase.projectCloneContainer

			image, err := GetProjectCloneImage(cheCtx)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedImage, image)
		})
	}
}
