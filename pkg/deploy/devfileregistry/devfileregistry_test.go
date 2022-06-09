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
package devfileregistry

import (
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

func TestDevfileRegistryReconcile(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	ctx := test.GetDeployContext(nil, []runtime.Object{})

	devfileregistry := NewDevfileRegistryReconciler()
	_, done, err := devfileregistry.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "devfile-registry", Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "devfile-registry", Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "devfile-registry", Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.NotEmpty(t, ctx.CheCluster.Status.DevfileRegistryURL)
}

func TestShouldSetUpCorrectlyDevfileRegistryURL(t *testing.T) {
	type testCase struct {
		name                       string
		initObjects                []runtime.Object
		cheCluster                 *chev2.CheCluster
		expectedDevfileRegistryURL string
	}

	testCases := []testCase{
		{
			name: "Test Status.DevfileRegistryURL #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
			expectedDevfileRegistryURL: "https://che-host/devfile-registry",
		},
		{
			name: "Test Status.DevfileRegistryURL #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
								{Url: "https://devfile-registry.external.2"},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
			expectedDevfileRegistryURL: "https://che-host/devfile-registry",
		},
		{
			name: "Test Status.DevfileRegistryURL #3",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							DisableInternalRegistry: true,
							ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
								{Url: "https://devfile-registry.external.2"},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
			expectedDevfileRegistryURL: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			devfileregistry := NewDevfileRegistryReconciler()
			devfileregistry.Reconcile(ctx)

			assert.Equal(t, ctx.CheCluster.Status.DevfileRegistryURL, testCase.expectedDevfileRegistryURL)
		})
	}
}
