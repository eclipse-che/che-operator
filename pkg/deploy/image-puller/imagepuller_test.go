//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package imagepuller

import (
	"context"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeDiscovery "k8s.io/client-go/discovery/fake"

	"testing"
)

func TestImagePullerConfiguration(t *testing.T) {
	type testCase struct {
		name                string
		cheCluster          *chev2.CheCluster
		initObjects         []runtime.Object
		expectedImagePuller *chev1alpha1.KubernetesImagePuller
	}

	testCases := []testCase{
		{
			name: "case #1: KubernetesImagePuller with defaults",
			cheCluster: InitCheCluster(chev2.ImagePuller{
				Enable: true,
			}),
			expectedImagePuller: InitImagePuller(chev1alpha1.KubernetesImagePullerSpec{
				DeploymentName:   defaultDeploymentName,
				ConfigMapName:    defaultConfigMapName,
				ImagePullerImage: defaultImagePullerImage,
				Images:           getDefaultImages(),
			}),
		},
		{
			name: "case #2: KubernetesImagePuller with custom configuration",
			cheCluster: InitCheCluster(chev2.ImagePuller{
				Enable: true,
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					ConfigMapName:    "custom-config-map",
					ImagePullerImage: "custom-image",
					DeploymentName:   "custom-deployment",
					Images:           "image=image_url;",
				}}),
			expectedImagePuller: InitImagePuller(chev1alpha1.KubernetesImagePullerSpec{
				ConfigMapName:    "custom-config-map",
				ImagePullerImage: "custom-image",
				DeploymentName:   "custom-deployment",
				Images:           "image=image_url;",
			}),
		},
		{
			name: "case #3: KubernetesImagePuller already exists",
			cheCluster: InitCheCluster(chev2.ImagePuller{
				Enable: true,
			}),
			initObjects: []runtime.Object{
				InitImagePuller(chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName:   defaultDeploymentName,
					ConfigMapName:    defaultConfigMapName,
					ImagePullerImage: defaultImagePullerImage,
				}),
			},
			expectedImagePuller: InitImagePuller(chev1alpha1.KubernetesImagePullerSpec{
				DeploymentName:   defaultDeploymentName,
				ConfigMapName:    defaultConfigMapName,
				ImagePullerImage: defaultImagePullerImage,
				Images:           getDefaultImages(),
			}),
		},
		{
			name: "case #4: KubernetesImagePuller already exists and updated with custom configuration",
			cheCluster: InitCheCluster(chev2.ImagePuller{
				Enable: true,
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					ConfigMapName:    "custom-config-map",
					ImagePullerImage: "custom-image",
					DeploymentName:   "custom-deployment",
					Images:           "image=image_url;",
				}}),
			initObjects: []runtime.Object{
				InitImagePuller(chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName:   defaultDeploymentName,
					ConfigMapName:    defaultConfigMapName,
					ImagePullerImage: defaultImagePullerImage,
				}),
			},
			expectedImagePuller: InitImagePuller(chev1alpha1.KubernetesImagePullerSpec{
				ConfigMapName:    "custom-config-map",
				ImagePullerImage: "custom-image",
				DeploymentName:   "custom-deployment",
				Images:           "image=image_url;",
			}),
		},
		{
			name: "case #5: Delete KubernetesImagePuller",
			cheCluster: InitCheCluster(chev2.ImagePuller{
				Enable: false,
			}),
			initObjects: []runtime.Object{
				InitImagePuller(chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName:   defaultDeploymentName,
					ConfigMapName:    defaultConfigMapName,
					ImagePullerImage: defaultImagePullerImage,
				}),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.cheCluster, testCase.initObjects)
			ctx.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
				{
					GroupVersion: "che.eclipse.org/v1alpha1",
					APIResources: []metav1.APIResource{
						{
							Name: resourceName,
						},
					},
				},
			}

			ip := NewImagePuller()
			_, _, err := ip.Reconcile(ctx)
			assert.NoError(t, err)

			actualImagePuller := &chev1alpha1.KubernetesImagePuller{}
			err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: "eclipse-che", Name: "eclipse-che-image-puller"}, actualImagePuller)
			if testCase.cheCluster.Spec.Components.ImagePuller.Enable {
				assert.NoError(t, err)

				diff := cmp.Diff(
					testCase.expectedImagePuller,
					actualImagePuller,
					cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion", "OwnerReferences"))
				if diff != "" {
					t.Errorf("Expected KubernetesImagePuller and KubernetesImagePuller returned from API server differ (-want, +got): %v", diff)
				}
			} else {
				assert.True(t, errors.IsNotFound(err))
			}
		})
	}
}

func InitCheCluster(imagePuller chev2.ImagePuller) *chev2.CheCluster {
	return &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "org.eclipse.che/v2",
			Kind:       "CheCluster",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				ImagePuller: imagePuller,
			},
		},
	}
}

func InitImagePuller(kubernetesImagePullerSpec chev1alpha1.KubernetesImagePullerSpec) *chev1alpha1.KubernetesImagePuller {
	return &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: chev1alpha1.GroupVersion.String(),
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che-image-puller",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				constants.KubernetesComponentLabelKey: constants.KubernetesImagePullerComponentName,
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
			},
		},
		Spec: kubernetesImagePullerSpec,
	}
}
