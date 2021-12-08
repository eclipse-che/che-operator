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
	"os"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

func TestDevfileRegistryReconcile(t *testing.T) {
	util.IsOpenShift = true
	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{})

	devfileregistry := NewDevfileRegistryReconciler()
	_, done, err := devfileregistry.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "devfile-registry", Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "devfile-registry", Namespace: "eclipse-che"}, &routev1.Route{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "devfile-registry", Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "devfile-registry", Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.NotEmpty(t, ctx.CheCluster.Status.DevfileRegistryURL)
}

func TestShouldSetUpCorrectlyDevfileRegistryURL(t *testing.T) {
	type testCase struct {
		name                       string
		isOpenShift                bool
		isOpenShift4               bool
		initObjects                []runtime.Object
		cheCluster                 *orgv1.CheCluster
		expectedDevfileRegistryURL string
	}

	testCases := []testCase{
		{
			name: "Test Status.DevfileRegistryURL #1",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      os.Getenv("CHE_FLAVOR"),
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ExternalDevfileRegistry: false,
					},
				},
			},
			expectedDevfileRegistryURL: "http://devfile-registry-eclipse-che./",
		},
		{
			name: "Test Status.DevfileRegistryURL #2",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      os.Getenv("CHE_FLAVOR"),
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ExternalDevfileRegistry: false,
						DevfileRegistryUrl:      "https://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "https://devfile-registry.external.2"},
						},
					},
				},
			},
			expectedDevfileRegistryURL: "http://devfile-registry-eclipse-che./",
		},
		{
			name: "Test Status.DevfileRegistryURL #2",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      os.Getenv("CHE_FLAVOR"),
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ExternalDevfileRegistry: true,
						DevfileRegistryUrl:      "https://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "https://devfile-registry.external.2"},
						},
					},
				},
			},
			expectedDevfileRegistryURL: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			devfileregistry := NewDevfileRegistryReconciler()
			devfileregistry.Reconcile(ctx)

			assert.Equal(t, ctx.CheCluster.Status.DevfileRegistryURL, testCase.expectedDevfileRegistryURL)
		})
	}
}
