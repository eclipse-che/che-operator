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
	"time"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
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
		terminateAfterCreate  *corev1.Pod
		expectedImage         string
	}

	now := time.Now()

	newDWOPod := func(name string, namespace string, image string, created time.Time) *corev1.Pod {
		pod := test.NewDWOPod(namespace, image)
		pod.Name = name
		pod.CreationTimestamp = metav1.NewTime(created)
		return pod
	}

	// The fake client strips DeletionTimestamp on create, so termination is
	// simulated by deleting a pod with finalizers after the context is built.
	terminatingPod := newDWOPod("dwo-a", "devworkspace-controller", "project-clone:new", now)
	terminatingPod.Finalizers = []string{"test"}

	failedPod := newDWOPod("dwo-a", "devworkspace-controller", "project-clone:new", now)
	failedPod.Status.Phase = corev1.PodFailed

	otherServiceAccountPod := newDWOPod("dwo-a", "devworkspace-controller", "project-clone:new", now)
	otherServiceAccountPod.Spec.ServiceAccountName = "default"

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
		{
			name: "Image from the newest pod during a rolling update",
			initObjects: []client.Object{
				newDWOPod("dwo-a", "devworkspace-controller", "project-clone:old", now.Add(-time.Hour)),
				newDWOPod("dwo-b", "devworkspace-controller", "project-clone:new", now),
			},
			expectedImage: "project-clone:new",
		},
		{
			name: "Terminating pod is skipped",
			initObjects: []client.Object{
				terminatingPod,
				newDWOPod("dwo-b", "devworkspace-controller", "project-clone:old", now.Add(-time.Hour)),
			},
			terminateAfterCreate: terminatingPod,
			expectedImage:        "project-clone:old",
		},
		{
			name: "Pod with another service account is skipped",
			initObjects: []client.Object{
				otherServiceAccountPod,
				newDWOPod("dwo-b", "devworkspace-controller", "project-clone:old", now.Add(-time.Hour)),
			},
			expectedImage: "project-clone:old",
		},
		{
			name: "Failed pod is skipped",
			initObjects: []client.Object{
				failedPod,
				newDWOPod("dwo-b", "devworkspace-controller", "project-clone:old", now.Add(-time.Hour)),
			},
			expectedImage: "project-clone:old",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx := test.NewCheContextBuilder().WithObjects(testCase.initObjects...).Build()
			if testCase.terminateAfterCreate != nil {
				assert.NoError(t, cheCtx.ClusterAPI.Client.Delete(cheCtx.Context, testCase.terminateAfterCreate))
			}

			cheCtx.CheCluster.Spec.DevEnvironments.ProjectCloneContainer = testCase.projectCloneContainer

			image, err := GetProjectCloneImage(cheCtx)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedImage, image)
		})
	}
}
