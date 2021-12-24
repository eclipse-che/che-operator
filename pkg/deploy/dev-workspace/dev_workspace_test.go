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
	DevWorkspaceCSVName = "devworkspace-operator.v0.9.0"
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
						CustomCheProperties:    map[string]string{"CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S": "true"},
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
						CustomCheProperties:    map[string]string{"CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S": "true"},
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

			util.IsOpenShift = testCase.IsOpenShift
			util.IsOpenShift4 = testCase.IsOpenShift4

			// reread templates (workaround after setting IsOpenShift value)
			DevWorkspaceTemplates = DevWorkspaceTemplatesPath()
			DevWorkspaceIssuerFile = DevWorkspaceTemplates + "/devworkspace-controller-selfsigned-issuer.Issuer.yaml"
			DevWorkspaceCertificateFile = DevWorkspaceTemplates + "/devworkspace-controller-serving-cert.Certificate.yaml"

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
	err := os.Setenv("ALLOW_DEVWORKSPACE_ENGINE", "false")
	assert.NoError(t, err)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.Nil(t, err, "Reconciliation error occurred %v", err)
	assert.True(t, done, "DevWorkspace should be reconciled.")
}

func TestReconcileWhenWebTerminalSubscriptionExists(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
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
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
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

	util.IsOpenShift = true
	util.IsOpenShift4 = true
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
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			// no che annotations are there
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
	err := deployContext.ClusterAPI.Client.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)

	util.IsOpenShift = true
	util.IsOpenShift4 = true
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
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				deploy.CheEclipseOrgNamespace: "eclipse-che",
			},
			// no che annotations are there
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
	err := deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)

	exists, err := deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace, Namespace: DevWorkspaceNamespace},
		&corev1.Namespace{})

	util.IsOpenShift = true
	util.IsOpenShift4 = true
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
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				deploy.CheEclipseOrgNamespace: "eclipse-che-removed",
			},
			// no che annotations are there
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(crdv1.SchemeGroupVersion, &crdv1.CustomResourceDefinition{})
	err := deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)

	exists, err := deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace, Namespace: DevWorkspaceNamespace},
		&corev1.Namespace{})

	util.IsOpenShift = true
	util.IsOpenShift4 = true
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
	assert.Equal(t, "eclipse-che", dwoNamespace.GetAnnotations()[deploy.CheEclipseOrgNamespace])

	// check that objects are sync
	exists, err = deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.True(t, exists, "DevWorkspace deployment is not created in Che managed DWO namespace")
	assert.NoError(t, err, "Failed to get devworkspace deployment")
}

func TestReconcileDevWorkspaceIfManagedDWOShouldNotBeTakenUnderControl(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "che-cluster",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}
	cheCluster2 := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che2",
			Name:      "che-cluster2",
		},
		Spec: orgv1.CheClusterSpec{
			DevWorkspace: orgv1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
		},
	}

	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			Annotations: map[string]string{
				deploy.CheEclipseOrgNamespace: "eclipse-che2",
			},
			// no che annotations are there
		},
	}
	deployContext := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
	deployContext.ClusterAPI.Scheme.AddKnownTypes(crdv1.SchemeGroupVersion, &crdv1.CustomResourceDefinition{})
	err := deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), dwoNamespace)
	assert.NoError(t, err)
	err = deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), cheCluster2)
	assert.NoError(t, err)

	exists, err := deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceNamespace, Namespace: DevWorkspaceNamespace},
		&corev1.Namespace{})

	util.IsOpenShift = true
	util.IsOpenShift4 = true
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
	assert.Equal(t, "eclipse-che2", dwoNamespace.GetAnnotations()[deploy.CheEclipseOrgNamespace])

	// check that objects are sync
	exists, err = deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.False(t, exists, "DevWorkspace deployment is not created in Che managed DWO namespace")
	assert.NoError(t, err, "Failed to get devworkspace deployment")
}
