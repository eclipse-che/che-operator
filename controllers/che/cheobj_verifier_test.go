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

package che

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/test/checontext"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsTrustedBundleConfigMap(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []client.Object
		objNamespace            string
		objLabels               map[string]string
		expectedIsEclipseCheObj bool
	}

	testObject := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"app.kubernetes.io/part-of":   "che.eclipse.org",
				"app.kubernetes.io/component": "ca-bundle",
				"app.kubernetes.io/instance":  defaults.GetCheFlavor(),
			},
		},
	}

	testCases := []testCase{
		{
			name:                    "Cluster scope object",
			initObjects:             []client.Object{},
			objNamespace:            "",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace",
			initObjects:             []client.Object{},
			objNamespace:            "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in 'eclipse-che' namespace, not ca-bundle component",
			initObjects:             []client.Object{},
			objLabels:               map[string]string{"app.kubernetes.io/part-of": "che.eclipse.org"},
			objNamespace:            "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che'",
			initObjects:             []client.Object{},
			objNamespace:            "test-eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Cluster scope object, all-namespaces mode",
			initObjects:             []client.Object{},
			objNamespace:            "",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace, all-namespaces mode",
			initObjects:             []client.Object{},
			objNamespace:            "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che', all-namespaces mode",
			initObjects:             []client.Object{},
			objNamespace:            "test-eclipse-che",
			expectedIsEclipseCheObj: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			test.StartOn(infrastructure.OpenShiftV4)
			cheCtx := checontext.NewBuilder().WithObjects(testCase.initObjects...).Build()

			newTestObject := testObject.DeepCopy()
			newTestObject.Namespace = testCase.objNamespace
			if testCase.objLabels != nil {
				newTestObject.Labels = testCase.objLabels
			}

			isEclipseCheObj, req := IsTrustedBundleConfigMap(cheCtx.Context, cheCtx.ClusterAPI.Client, newTestObject)

			assert.Equal(t, testCase.expectedIsEclipseCheObj, isEclipseCheObj)
			if isEclipseCheObj {
				assert.Equal(t, req.Namespace, cheCtx.CheCluster.Namespace)
				assert.Equal(t, req.Name, cheCtx.CheCluster.Name)
			}
		})
	}
}

func TestIsEclipseCheRelatedObj(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []client.Object
		objNamespace            string
		expectedIsEclipseCheObj bool
	}

	testObject := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"app.kubernetes.io/part-of":  "che.eclipse.org",
				"app.kubernetes.io/instance": defaults.GetCheFlavor(),
			},
		},
	}

	testCases := []testCase{
		{
			name:                    "Cluster scope object",
			initObjects:             []client.Object{},
			objNamespace:            "",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace",
			initObjects:             []client.Object{},
			objNamespace:            "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che'",
			initObjects:             []client.Object{},
			objNamespace:            "test-eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Cluster scope object, all-namespaces mode",
			initObjects:             []client.Object{},
			objNamespace:            "",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace, all-namespaces mode",
			initObjects:             []client.Object{},
			objNamespace:            "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che', all-namespaces mode",
			initObjects:             []client.Object{},
			objNamespace:            "test-eclipse-che",
			expectedIsEclipseCheObj: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			test.StartOn(infrastructure.OpenShiftV4)
			cheCtx := checontext.NewBuilder().WithObjects(testCase.initObjects...).Build()

			testObject.Namespace = testCase.objNamespace
			isEclipseCheObj, req := IsEclipseCheRelatedObj(cheCtx.Context, cheCtx.ClusterAPI.Client, testObject)

			assert.Equal(t, testCase.expectedIsEclipseCheObj, isEclipseCheObj)
			if isEclipseCheObj {
				assert.Equal(t, req.Namespace, cheCtx.CheCluster.Namespace)
				assert.Equal(t, req.Name, cheCtx.CheCluster.Name)
			}
		})
	}
}

func TestMapDevWorkspaceOperatorPodToCheCluster(t *testing.T) {
	type testCase struct {
		name                              string
		obj                               client.Object
		expectedIsDevWorkspaceOperatorPod bool
	}

	testCases := []testCase{
		{
			name:                              "DevWorkspace Operator controller pod",
			obj:                               test.NewDWOPod("devworkspace-controller", "project-clone:dwo"),
			expectedIsDevWorkspaceOperatorPod: true,
		},
		{
			name:                              "DevWorkspace Operator webhook server pod",
			obj:                               newDevWorkspaceOperatorWebhookServerPod(),
			expectedIsDevWorkspaceOperatorPod: false,
		},
		{
			name: "ConfigMap with DevWorkspace Operator controller labels",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "devworkspace-controller",
					Labels:    test.NewDWOPod("devworkspace-controller", "project-clone:dwo").Labels,
				},
			},
			expectedIsDevWorkspaceOperatorPod: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cheCluster := &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"},
			}
			test.StartOn(infrastructure.OpenShiftV4)
			cheCtx := checontext.NewBuilder().WithCheCluster(cheCluster).Build()

			isDevWorkspaceOperatorPod, request := mapDevWorkspaceOperatorPodToCheCluster(
				cheCtx.Context,
				cheCtx.ClusterAPI.Client,
				testCase.obj,
			)

			assert.Equal(t, testCase.expectedIsDevWorkspaceOperatorPod, isDevWorkspaceOperatorPod)
			if testCase.expectedIsDevWorkspaceOperatorPod {
				assert.Equal(t, "eclipse-che", request.Namespace)
				assert.Equal(t, "eclipse-che", request.Name)
			}
		})
	}
}

func TestMapDevWorkspaceOperatorPodToCheClusterWithoutCheCluster(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)
	cheCtx := checontext.NewBuilder().WithCheCluster(nil).Build()

	isDevWorkspaceOperatorPod, _ := mapDevWorkspaceOperatorPodToCheCluster(
		cheCtx.Context,
		cheCtx.ClusterAPI.Client,
		test.NewDWOPod("devworkspace-controller", "project-clone:dwo"),
	)

	assert.False(t, isDevWorkspaceOperatorPod)
}

func newDevWorkspaceOperatorWebhookServerPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-webhook-server",
			Namespace: "devworkspace-controller",
			Labels: map[string]string{
				constants.KubernetesNameLabelKey:   "devworkspace-webhook-server",
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
			},
		},
	}
}
