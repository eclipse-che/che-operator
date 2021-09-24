//
// Copyright (c) 2021 Red Hat, Inc.
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
	"context"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"strings"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DevWorkspaceCSVName = "devworkspace-operator.v0.6.0"
)

func TestReconcileDevWorkspace(t *testing.T) {
	type testCase struct {
		name         string
		IsOpenShift  bool
		IsOpenShift4 bool
		cheCluster   *orgv1.CheCluster
	}

	testCases := []testCase{
		{
			name: "Reconcile DevWorkspace on OpenShift",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
						Enable: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(true),
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "single-host",
					},
				},
			},
			IsOpenShift:  true,
			IsOpenShift4: true,
		},
		{
			name: "Reconcile DevWorkspace on K8S multi-host",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
						Enable: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(true),
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "multi-host",
					},
					K8s: orgv1.CheClusterSpecK8SOnly{
						IngressDomain: "che.domain",
					},
				},
			},
			IsOpenShift:  false,
			IsOpenShift4: false,
		},
		{
			name: "Reconcile DevWorkspace on K8S single-host",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
						Enable: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(true),
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "single-host",
					},
					K8s: orgv1.CheClusterSpecK8SOnly{
						IngressDomain: "che.domain",
					},
				},
			},
			IsOpenShift:  false,
			IsOpenShift4: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})
			deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})

			util.IsOpenShift = testCase.IsOpenShift
			util.IsOpenShift4 = testCase.IsOpenShift4
			os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")

			done, err := ReconcileDevWorkspace(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace operator has not been provisioned")
		})
	}
}

func TestShouldNotReconcileDevWorkspaceIfForbidden(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "false")

	reconciled, err := ReconcileDevWorkspace(deployContext)

	assert.False(t, reconciled, "DevWorkspace should not be reconciled")
	assert.NotNil(t, err, "Error expected")
	assert.True(t, strings.Contains(err.Error(), "deploy Eclipse Che from tech-preview channel"), "Unrecognized error occurred %v", err)
}

func TestShouldReconcileDevWorkspaceIfDevWorkspaceDeploymentExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	devworkspaceDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DevWorkspaceDeploymentName,
			Namespace: DevWorkspaceNamespace,
		},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{devworkspaceDeployment})

	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "false")

	reconciled, err := ReconcileDevWorkspace(deployContext)

	assert.Nil(t, err, "Reconciliation error occurred %v", err)
	assert.True(t, reconciled, "Devworkspace should be reconciled.")
}

func TestReconcileDevWorkspaceShouldNotInstallDWOIfWebTerminalSubscriptionExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(true),
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "single-host",
			},
		},
	}
	subscription := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: WebTerminalOperatorNamespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{subscription})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(admissionregistrationv1.SchemeGroupVersion, &admissionregistrationv1.MutatingWebhookConfiguration{})
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: SubscriptionResourceName},
			},
		},
	}
	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")

	isDone, err := ReconcileDevWorkspace(deployContext)
	assert.NoError(t, err)
	assert.True(t, isDone)

	// verify that DWO is not provisioned
	namespace := &corev1.Namespace{}
	err = deployContext.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{Name: DevWorkspaceNamespace}, namespace)
	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestReconcileDevWorkspaceCheckIfCSVExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(true),
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "single-host",
			},
		},
	}
	devWorkspaceCSV := &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DevWorkspaceCSVName,
			Namespace: "openshift-operators",
		},
		Spec: operatorsv1alpha1.ClusterServiceVersionSpec{},
	}

	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.ClusterServiceVersion{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.ClusterServiceVersionList{})
	deployContext.ClusterAPI.Client.Create(context.TODO(), devWorkspaceCSV)
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{
					Name: ClusterServiceVersionResourceName,
				},
			},
		},
	}

	util.IsOpenShift = true
	util.IsOpenShift4 = true
	os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	reconciled, _ := ReconcileDevWorkspace(deployContext)

	assert.True(t, reconciled, "Reconcile is not triggered")

	// Get Devworkspace namespace. If error is thrown means devworkspace is not anymore installed if CSV is detected
	err := deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: DevWorkspaceNamespace}, &corev1.Namespace{})
	assert.True(t, k8sErrors.IsNotFound(err), "DevWorkspace namespace is created when instead DWO CSV is expected to be created")
}
