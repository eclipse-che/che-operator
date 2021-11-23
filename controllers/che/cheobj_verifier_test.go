//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"os"
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestIsTrustedBundleConfigMap(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []runtime.Object
		checluster              *orgv1.CheCluster
		objNamespace            string
		objLabels               map[string]string
		watchNamespace          string
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
				"app.kubernetes.io/instance":  os.Getenv("CHE_FLAVOR"),
			},
		},
	}

	testCases := []testCase{
		{
			name:                    "Cluster scope object",
			initObjects:             []runtime.Object{},
			objNamespace:            "",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace",
			initObjects:             []runtime.Object{},
			objNamespace:            "eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in 'eclipse-che' namespace, not ca-bundle component",
			initObjects:             []runtime.Object{},
			objLabels:               map[string]string{"app.kubernetes.io/part-of": "che.eclipse.org"},
			objNamespace:            "eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:        "Object in 'eclipse-che' namespace, not ca-bundle component, but trusted configmap",
			initObjects: []runtime.Object{},
			checluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ServerTrustStoreConfigMapName: "test",
					},
				},
			},
			objLabels:               map[string]string{},
			objNamespace:            "eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che'",
			initObjects:             []runtime.Object{},
			objNamespace:            "test-eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name: "Object in 'eclipse-che' namespace, several checluster CR",
			initObjects: []runtime.Object{
				// checluster CR in `default` namespace
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "default"}},
			},
			objNamespace:            "eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Cluster scope object, all-namespaces mode",
			initObjects:             []runtime.Object{},
			objNamespace:            "",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace, all-namespaces mode",
			initObjects:             []runtime.Object{},
			objNamespace:            "eclipse-che",
			watchNamespace:          "",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che', all-namespaces mode",
			initObjects:             []runtime.Object{},
			objNamespace:            "test-eclipse-che",
			watchNamespace:          "",
			expectedIsEclipseCheObj: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(testCase.checluster, testCase.initObjects)

			newTestObject := testObject.DeepCopy()
			newTestObject.ObjectMeta.Namespace = testCase.objNamespace
			if testCase.objLabels != nil {
				newTestObject.ObjectMeta.Labels = testCase.objLabels
			}

			isEclipseCheObj, req := IsTrustedBundleConfigMap(deployContext.ClusterAPI.NonCachingClient, testCase.watchNamespace, newTestObject)

			assert.Equal(t, testCase.expectedIsEclipseCheObj, isEclipseCheObj)
			if isEclipseCheObj {
				assert.Equal(t, req.Namespace, deployContext.CheCluster.Namespace)
				assert.Equal(t, req.Name, deployContext.CheCluster.Name)
			}
		})
	}
}

func TestIsEclipseCheRelatedObj(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []runtime.Object
		objNamespace            string
		watchNamespace          string
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
				"app.kubernetes.io/instance": os.Getenv("CHE_FLAVOR"),
			},
		},
	}

	testCases := []testCase{
		{
			name:                    "Cluster scope object",
			initObjects:             []runtime.Object{},
			objNamespace:            "",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace",
			initObjects:             []runtime.Object{},
			objNamespace:            "eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che'",
			initObjects:             []runtime.Object{},
			objNamespace:            "test-eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name: "Object in 'eclipse-che' namespace, several checluster CR",
			initObjects: []runtime.Object{
				// checluster CR in `default` namespace
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "default"}},
			},
			objNamespace:            "eclipse-che",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Cluster scope object, all-namespaces mode",
			initObjects:             []runtime.Object{},
			objNamespace:            "",
			watchNamespace:          "eclipse-che",
			expectedIsEclipseCheObj: false,
		},
		{
			name:                    "Object in 'eclipse-che' namespace, all-namespaces mode",
			initObjects:             []runtime.Object{},
			objNamespace:            "eclipse-che",
			watchNamespace:          "",
			expectedIsEclipseCheObj: true,
		},
		{
			name:                    "Object in another namespace than 'eclipse-che', all-namespaces mode",
			initObjects:             []runtime.Object{},
			objNamespace:            "test-eclipse-che",
			watchNamespace:          "",
			expectedIsEclipseCheObj: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(nil, testCase.initObjects)

			testObject.ObjectMeta.Namespace = testCase.objNamespace
			isEclipseCheObj, req := IsEclipseCheRelatedObj(deployContext.ClusterAPI.NonCachingClient, testCase.watchNamespace, testObject)

			assert.Equal(t, testCase.expectedIsEclipseCheObj, isEclipseCheObj)
			if isEclipseCheObj {
				assert.Equal(t, req.Namespace, deployContext.CheCluster.Namespace)
				assert.Equal(t, req.Name, deployContext.CheCluster.Name)
			}
		})
	}
}
