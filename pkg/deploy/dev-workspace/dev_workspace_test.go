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
package devworkspace

import (
	"context"
	"os"

	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
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
	DevWorkspaceCSVName = "devworkspace-operator.v0.11.0"
)

func TestReconcileDevWorkspace(t *testing.T) {
	type testCase struct {
		name           string
		infrastructure infrastructure.Type
		cheCluster     *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name: "Reconcile DevWorkspace on OpenShift",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
			infrastructure: infrastructure.OpenShiftv4,
		},
		{
			name: "Reconcile DevWorkspace on K8S",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{"CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S": "true"},
						},
					},
					Networking: chev2.CheClusterSpecNetworking{
						Domain: "che.domain",
					},
				},
			},
			infrastructure: infrastructure.Kubernetes,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			infrastructure.InitializeForTesting(testCase.infrastructure)

			err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
			assert.NoError(t, err)

			devWorkspaceReconciler := NewDevWorkspaceReconciler()
			_, done, err := devWorkspaceReconciler.Reconcile(deployContext)
			assert.NoError(t, err, "Reconcile failed")
			assert.True(t, done, "Dev Workspace operator has not been provisioned")
		})
	}
}

func TestShouldReconcileDevWorkspaceIfDevWorkspaceDeploymentExists(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
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

	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{devworkspaceDeployment})

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "false")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.Nil(t, err, "Reconciliation error occurred %v", err)
	assert.True(t, done, "DevWorkspace should be reconciled.")
}

func TestReconcileWhenWebTerminalSubscriptionExists(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}
	subscription := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{},
	}

	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{subscription})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(admissionregistrationv1.SchemeGroupVersion, &admissionregistrationv1.MutatingWebhookConfiguration{})
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: SubscriptionResourceName},
			},
		},
	}

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.NoError(t, err)
	assert.True(t, done)

	// verify that DWO is not provisioned
	namespace := &corev1.Namespace{}
	err = deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Name: DevWorkspaceNamespace}, namespace)
	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestReconcileDevWorkspaceCheckIfCSVExists(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}
	devWorkspaceCSV := &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DevWorkspaceCSVName,
			Namespace: "openshift-operators",
		},
		Spec: operatorsv1alpha1.ClusterServiceVersionSpec{},
	}

	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.ClusterServiceVersion{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.ClusterServiceVersionList{})
	err := deployContext.ClusterAPI.Client.Create(context.TODO(), devWorkspaceCSV)
	assert.NoError(t, err)
	deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{
					Name: ClusterServiceVersionResourceName,
				},
			},
		},
	}

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	err = os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done, "Reconcile is not triggered")

	// Get Devworkspace namespace. If error is thrown means devworkspace is not anymore installed if CSV is detected
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: DevWorkspaceNamespace}, &corev1.Namespace{})
	assert.True(t, k8sErrors.IsNotFound(err), "DevWorkspace namespace is created when instead DWO CSV is expected to be created")
}

func TestReconcileDevWorkspaceIfUnmanagedDWONamespaceExists(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			// no che annotations are there
		},
	}
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})
	err := deployContext.ClusterAPI.Client.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	err = os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done, "Reconcile is not triggered")

	// check is reconcile created deployment if existing namespace is not annotated in che specific way
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: DevWorkspaceDeploymentName}, &appsv1.Deployment{})
	assert.True(t, k8sErrors.IsNotFound(err), "DevWorkspace deployment is created but it should not since it's DWO namespace managed not by Che")
}

func TestReconcileDevWorkspaceIfManagedDWONamespaceExists(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: "eclipse-che",
			},
			// no che annotations are there
		},
	}
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})
	err := deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)

	exists, err := deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace, Namespace: DevWorkspaceNamespace},
		&corev1.Namespace{})

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	err = os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done, "Reconcile is not triggered")
	assert.NoError(t, err, "Reconcile failed")

	// check is reconcile created deployment if existing namespace is not annotated in che specific way
	exists, err = deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.True(t, exists, "DevWorkspace deployment is not created in Che managed DWO namespace")
	assert.NoError(t, err, "Failed to get devworkspace deployment")
}

func TestReconcileDevWorkspaceIfManagedDWOShouldBeTakenUnderControl(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: "eclipse-che-removed",
			},
			// no che annotations are there
		},
	}
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(crdv1.SchemeGroupVersion, &crdv1.CustomResourceDefinition{})
	err := deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)

	exists, err := deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace, Namespace: DevWorkspaceNamespace},
		&corev1.Namespace{})

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	err = os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done, "Reconcile is not triggered")
	assert.NoError(t, err, "Reconcile failed")

	// check is reconcile updated namespace with according way
	exists, err = deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace},
		dwoNamespace)
	assert.True(t, exists, "DevWorkspace Namespace does not exist")
	assert.Equal(t, "eclipse-che", dwoNamespace.GetAnnotations()[constants.CheEclipseOrgNamespace])

	// check that objects are sync
	exists, err = deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.True(t, exists, "DevWorkspace deployment is not created in Che managed DWO namespace")
	assert.NoError(t, err, "Failed to get devworkspace deployment")
}

func TestReconcileDevWorkspaceIfManagedDWOShouldNotBeTakenUnderControl(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "che-cluster",
		},
	}
	cheCluster2 := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che2",
			Name:      "che-cluster2",
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: "eclipse-che2",
			},
			// no che annotations are there
		},
	}
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(crdv1.SchemeGroupVersion, &crdv1.CustomResourceDefinition{})
	err := deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)
	err = deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), cheCluster2)
	assert.NoError(t, err)

	exists, err := deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace, Namespace: DevWorkspaceNamespace},
		&corev1.Namespace{})

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	err = os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "true")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done, "Reconcile is not triggered")
	assert.NoError(t, err, "Reconcile failed")

	// check is reconcile updated namespace with according way
	exists, err = deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace},
		dwoNamespace)
	assert.True(t, exists, "DevWorkspace Namespace does not exist")
	assert.Equal(t, "eclipse-che2", dwoNamespace.GetAnnotations()[constants.CheEclipseOrgNamespace])

	// check that objects are sync
	exists, err = deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.False(t, exists, "DevWorkspace deployment is not created in Che managed DWO namespace")
	assert.NoError(t, err, "Failed to get devworkspace deployment")
}
