//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
	"testing"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDevfileRegistryReconciler(t *testing.T) {
	defaultExternalDevfileRegistriesEnvVar := os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES")
	defer func() {
		_ = os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES", defaultExternalDevfileRegistriesEnvVar)
	}()

	_ = os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES", "[{\"url\": \"https://registry.devfile.io\"}]")

	// re initialize defaults with new env var
	defaults.Initialize()

	type testCase struct {
		name                              string
		cheCluster                        *chev2.CheCluster
		expectedDisableInternalRegistry   bool
		expectedExternalDevfileRegistries []chev2.ExternalDevfileRegistry
		expectedDevfileRegistryURL        string
	}

	testCases := []testCase{
		{
			name: "DisableInternalRegistry=false #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							DisableInternalRegistry: false,
						},
					},
				},
				Status: chev2.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry:8080",
				},
			},
			expectedDisableInternalRegistry:   false,
			expectedExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{{Url: "https://registry.devfile.io"}},
			expectedDevfileRegistryURL:        "",
		},
		{
			name: "DisableInternalRegistry=false #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							DisableInternalRegistry: false,
							ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
								{Url: "my-external-registry"},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry:8080",
				},
			},
			expectedDisableInternalRegistry: false,
			expectedExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
				{Url: "my-external-registry"},
				{Url: "https://registry.devfile.io"},
			},
			expectedDevfileRegistryURL: "",
		},
		{
			name: "DisableInternalRegistry=false #3",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							DisableInternalRegistry: false,
							ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
								{Url: "https://registry.devfile.io"},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry:8080",
				},
			},
			expectedDisableInternalRegistry: false,
			expectedExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
				{Url: "https://registry.devfile.io"},
			},
			expectedDevfileRegistryURL: "",
		},
		{
			name: "DisableInternalRegistry=true #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							DisableInternalRegistry: true,
						},
					},
				},
				Status: chev2.CheClusterStatus{},
			},
			expectedDisableInternalRegistry:   true,
			expectedExternalDevfileRegistries: nil,
			expectedDevfileRegistryURL:        "",
		},
		{
			name: "DisableInternalRegistry=true #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							DisableInternalRegistry:   true,
							ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{{Url: "my-external-registry"}},
						},
					},
				},
				Status: chev2.CheClusterStatus{},
			},
			expectedDisableInternalRegistry:   true,
			expectedExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{{Url: "my-external-registry"}},
			expectedDevfileRegistryURL:        "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			devfileRegistryReconciler := NewDevfileRegistryReconciler()
			for i := 0; i < 2; i++ {
				_, done, err := devfileRegistryReconciler.Reconcile(ctx)
				assert.True(t, done)
				assert.Nil(t, err)
			}

			assert.Equal(t, testCase.expectedDevfileRegistryURL, ctx.CheCluster.Status.DevfileRegistryURL)
			assert.Equal(t, testCase.expectedDisableInternalRegistry, ctx.CheCluster.Spec.Components.DevfileRegistry.DisableInternalRegistry)
			assert.Equal(t, len(testCase.expectedExternalDevfileRegistries), len(ctx.CheCluster.Spec.Components.DevfileRegistry.ExternalDevfileRegistries))
			assert.Equal(t, testCase.expectedExternalDevfileRegistries, ctx.CheCluster.Spec.Components.DevfileRegistry.ExternalDevfileRegistries)
		})
	}
}
