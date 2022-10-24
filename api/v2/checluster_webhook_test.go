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
package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVSXUrl(t *testing.T) {
	type testCase struct {
		name               string
		cheCluster         *CheCluster
		expectedOpenVSXUrl string
	}

	testCases := []testCase{
		{
			name: "Should set default openVSXURL when deploying Eclipse Che",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: CheClusterStatus{
					CheVersion: "",
				},
			},
			expectedOpenVSXUrl: openVSXDefaultUrl,
		},
		{
			name: "Should not set default openVSXURL when deploying Eclipse Che in AirGap environment",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: CheClusterSpec{
					ContainerRegistry: CheClusterContainerRegistry{
						Hostname: "hostname",
					},
				},
				Status: CheClusterStatus{
					CheVersion: "",
				},
			},
			expectedOpenVSXUrl: "",
		},
		{
			name: "Should not update openVSXURL for next version #1",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedOpenVSXUrl: "",
		},
		{
			name: "Should not update openVSXURL for next version #2",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: CheClusterSpec{
					Components: CheClusterComponents{
						PluginRegistry: PluginRegistry{
							OpenVSXURL: "open-vsx-url",
						},
					},
				},
				Status: CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedOpenVSXUrl: "open-vsx-url",
		},
		{
			name: "Should set default openVSXURL if Eclipse Che version is less then 7.53.0",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: CheClusterStatus{
					CheVersion: "7.52.2",
				},
			},
			expectedOpenVSXUrl: openVSXDefaultUrl,
		},
		{
			name: "Should not update openVSXURL if Eclipse Che version is less then 7.53.0",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: CheClusterSpec{
					Components: CheClusterComponents{
						PluginRegistry: PluginRegistry{
							OpenVSXURL: "open-vsx-url",
						},
					},
				},
				Status: CheClusterStatus{
					CheVersion: "7.52.2",
				},
			},
			expectedOpenVSXUrl: "open-vsx-url",
		},
		{
			name: "Should not update default openVSXURL if Eclipse Che version is greater or equal to 7.53.0 #1",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: CheClusterStatus{
					CheVersion: "7.53.0",
				},
			},
			expectedOpenVSXUrl: "",
		},
		{
			name: "Should not update default openVSXURL if Eclipse Che version is greater or equal to 7.53.0 #2",
			cheCluster: &CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: CheClusterSpec{
					Components: CheClusterComponents{
						PluginRegistry: PluginRegistry{
							OpenVSXURL: "open-vsx-url",
						},
					},
				},
				Status: CheClusterStatus{
					CheVersion: "7.53.0",
				},
			},
			expectedOpenVSXUrl: "open-vsx-url",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setOpenVSXUrl(testCase.cheCluster)
			assert.Equal(t, testCase.expectedOpenVSXUrl, testCase.cheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
		})
	}
}
