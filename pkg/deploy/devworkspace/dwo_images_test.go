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
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetProjectCloneImage(t *testing.T) {
	type testCase struct {
		name                  string
		projectCloneContainer *chev2.Container
		initObjects           []client.Object
		expectedImage         string
	}

	testCases := []testCase{
		{
			name:          "Image from DevWorkspace Operator pod",
			initObjects:   []client.Object{test.NewDWOPod("devworkspace-controller", "project-clone:dwo")},
			expectedImage: "project-clone:dwo",
		},
		{
			name:                  "Image from CheCluster",
			projectCloneContainer: &chev2.Container{Image: "project-clone:che"},
			initObjects:           []client.Object{test.NewDWOPod("devworkspace-controller", "project-clone:dwo")},
			expectedImage:         "project-clone:che",
		},
		{
			name:          "No DevWorkspace Operator pod",
			expectedImage: "",
		},
		{
			name:          "DevWorkspace Operator pod in another namespace",
			initObjects:   []client.Object{test.NewDWOPod("other-namespace", "project-clone:dwo")},
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
