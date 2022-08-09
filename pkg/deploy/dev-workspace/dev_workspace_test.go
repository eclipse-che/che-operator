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
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestShouldNotReconcileDevWorkspaceOnOpenShift(t *testing.T) {
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor() + "-operator",
			Namespace: "eclipse-che",
		},
	}

	deployContext := test.GetDeployContext(nil, []runtime.Object{cheOperatorDeployment})
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)
	assert.True(t, done)
	assert.Nil(t, err)

	err = deployContext.ClusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: DevWorkspaceNamespace}, &corev1.Namespace{})
	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestShouldReconcileDevWorkspaceOnKubernetes(t *testing.T) {
	cheOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor() + "-operator",
			Namespace: "eclipse-che",
		},
	}

	deployContext := test.GetDeployContext(nil, []runtime.Object{cheOperatorDeployment})
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)
	assert.True(t, done)
	assert.Nil(t, err)

	err = deployContext.ClusterAPI.Client.Get(context.TODO(),
		client.ObjectKey{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.Nil(t, err)
}

func TestShouldReconcileDevWorkspaceIfDevWorkspaceDeploymentExists(t *testing.T) {
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

	deployContext := test.GetDeployContext(nil, []runtime.Object{devworkspaceDeployment, cheOperatorDeployment})
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.Nil(t, err)
	assert.True(t, done)
}

func TestShouldNotReconcileDevWorkspaceIfUnmanagedDWONamespaceExists(t *testing.T) {
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
	deployContext := test.GetDeployContext(nil, []runtime.Object{cheOperatorDeployment, dwoNamespace})
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done)
	assert.Nil(t, err)

	err = deployContext.ClusterAPI.Client.Get(context.TODO(),
		client.ObjectKey{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.True(t, k8sErrors.IsNotFound(err))
}

func TestReconcileDevWorkspaceIfManagedDWONamespaceExists(t *testing.T) {
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
	deployContext := test.GetDeployContext(nil, []runtime.Object{cheOperatorDeployment, dwoNamespace})
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	devWorkspaceReconciler := NewDevWorkspaceReconciler()
	_, done, err := devWorkspaceReconciler.Reconcile(deployContext)

	assert.True(t, done)
	assert.Nil(t, err)

	err = deployContext.ClusterAPI.Client.Get(context.TODO(),
		client.ObjectKey{Name: DevWorkspaceDeploymentName, Namespace: DevWorkspaceNamespace},
		&appsv1.Deployment{})
	assert.Nil(t, err)
}
