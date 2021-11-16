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
package util

import (
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestFindCheCRinNamespace(t *testing.T) {
	type testCase struct {
		name              string
		initObjects       []runtime.Object
		watchNamespace    string
		expectedNumber    int
		expectedNamespace string
		expectedErr       bool
	}

	testCases := []testCase{
		{
			name: "CR in 'eclipse-che' namespace",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
			},
			watchNamespace:    "eclipse-che",
			expectedNumber:    1,
			expectedErr:       false,
			expectedNamespace: "eclipse-che",
		},
		{
			name: "CR in 'default' namespace",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "default"}},
			},
			watchNamespace: "eclipse-che",
			expectedNumber: 0,
			expectedErr:    true,
		},
		{
			name: "several CR in 'eclipse-che' namespace",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "test-eclipse-che", Namespace: "eclipse-che"}},
			},
			watchNamespace: "eclipse-che",
			expectedNumber: 2,
			expectedErr:    true,
		},
		{
			name: "several CR in different namespaces",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "default"}},
			},
			watchNamespace:    "eclipse-che",
			expectedNumber:    1,
			expectedErr:       false,
			expectedNamespace: "eclipse-che",
		},
		{
			name: "CR in 'eclipse-che' namespace, all-namespace mode",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
			},
			watchNamespace:    "",
			expectedNumber:    1,
			expectedErr:       false,
			expectedNamespace: "eclipse-che",
		},
		{
			name: "CR in 'default' namespace, all-namespace mode",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "default"}},
			},
			watchNamespace:    "",
			expectedNumber:    1,
			expectedErr:       false,
			expectedNamespace: "default",
		},
		{
			name: "several CR in 'eclipse-che' namespace, all-namespace mode",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "test-eclipse-che", Namespace: "eclipse-che"}},
			},
			watchNamespace: "",
			expectedNumber: 2,
			expectedErr:    true,
		},
		{
			name: "several CR in different namespaces, all-namespace mode",
			initObjects: []runtime.Object{
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "eclipse-che"}},
				&orgv1.CheCluster{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che", Namespace: "default"}},
			},
			watchNamespace: "",
			expectedNumber: 2,
			expectedErr:    true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)

			checluster, num, err := FindCheClusterCRInNamespace(cli, testCase.watchNamespace)
			assert.Equal(t, testCase.expectedNumber, num)
			if testCase.expectedErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			if num == 1 {
				assert.Equal(t, testCase.expectedNamespace, checluster.Namespace)
			}
		})
	}
}
