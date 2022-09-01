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
package pluginregistry

import (
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestOpenVSXUrlReconciler(t *testing.T) {
	type testCase struct {
		name               string
		cheCluster         *chev2.CheCluster
		expectedOpenVSXUrl string
	}

	testCases := []testCase{
		{
			name: "Should set default openVSXURL when deploying Eclipse Che",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "",
				},
			},
			expectedOpenVSXUrl: openVSXDefaultUrl,
		},
		{
			name: "Should not update openVSXURL for next version #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedOpenVSXUrl: "",
		},
		{
			name: "Should not update openVSXURL for next version #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: "open-vsx-url",
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedOpenVSXUrl: "open-vsx-url",
		},
		{
			name: "Should set default openVSXURL if Eclipse Che version is less then 7.52.0",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "7.51.2",
				},
			},
			expectedOpenVSXUrl: openVSXDefaultUrl,
		},
		{
			name: "Should not update openVSXURL if Eclipse Che version is less then 7.52.0",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: "open-vsx-url",
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "7.51.2",
				},
			},
			expectedOpenVSXUrl: "open-vsx-url",
		},
		{
			name: "Should not update default openVSXURL if Eclipse Che version is greater or equal to 7.53.0 #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "7.53.0",
				},
			},
			expectedOpenVSXUrl: "",
		},
		{
			name: "Should not update default openVSXURL if Eclipse Che version is greater or equal to 7.53.0 #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							OpenVSXURL: "open-vsx-url",
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheVersion: "7.53.0",
				},
			},
			expectedOpenVSXUrl: "open-vsx-url",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			openVSXUrlReconciler := NewOpenVSXUrlReconciler()
			_, _, err := openVSXUrlReconciler.Reconcile(ctx)

			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedOpenVSXUrl, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
		})
	}
}
