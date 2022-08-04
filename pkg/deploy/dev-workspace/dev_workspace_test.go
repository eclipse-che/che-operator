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
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"k8s.io/apimachinery/pkg/types"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileDevWorkspace(t *testing.T) {
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor() + "-operator",
			Namespace: "eclipse-che",
		},
	}

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
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{cheOperatorDeployment})
			infrastructure.InitializeForTesting(testCase.infrastructure)

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
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor() + "-operator",
			Namespace: "eclipse-che",
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
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperator,
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceController,
			},
		},
	}

	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{devworkspaceDeployment, cheOperatorDeployment})
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.Nil(t, err, "Reconciliation error occurred %v", err)
	assert.True(t, done, "DevWorkspace should be reconciled.")
}

func TestShouldNotReconcileDevWorkspaceIfDevWorkspaceDeploymentManagedByOLM(t *testing.T) {
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor() + "-operator",
			Namespace: "eclipse-che",
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
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperator,
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceController,
			},
			OwnerReferences: []metav1.OwnerReference{{}},
		},
	}
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}

	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{cheOperatorDeployment, devworkspaceDeployment})
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done)

	// verify that DWO is not provisioned
	err = deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Name: DevWorkspaceNamespace}, &corev1.Namespace{})
	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestShouldNotReconcileDevWorkspaceIfCheOperatorDeploymentManagedByOLM(t *testing.T) {
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            defaults.GetCheFlavor() + "-operator",
			Namespace:       "eclipse-che",
			OwnerReferences: []metav1.OwnerReference{{}},
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
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperator,
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceController,
			},
		},
	}
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}

	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{cheOperatorDeployment, devworkspaceDeployment})
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done)

	// verify that DWO is not provisioned
	err = deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Name: DevWorkspaceNamespace}, &corev1.Namespace{})
	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestShouldNotReconcileDevWorkspaceIfUnmanagedDWONamespaceExists(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor() + "-operator",
			Namespace: "eclipse-che",
		},
	}
	dwoNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceNamespace,
			// no che annotations are there
		},
	}
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{cheOperatorDeployment, dwoNamespace})
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

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
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor() + "-operator",
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
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{cheOperatorDeployment, dwoNamespace})
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done, "Reconcile is not triggered")
	assert.NoError(t, err, "Reconcile failed")

	// check is reconcile created deployment if existing namespace is not annotated in che specific way
	exists, err := deploy.Get(deployContext,
		types.NamespacedName{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.True(t, exists, "DevWorkspace deployment is not created in Che managed DWO namespace")
	assert.NoError(t, err, "Failed to get devworkspace deployment")
}
