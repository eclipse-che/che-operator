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
package dashboard

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

const Namespace = "eclipse-che"

func TestDashboardOpenShift(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      "eclipse-che",
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	routev1.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)
	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	util.IsOpenShift = true

	dashboard := NewDashboard(deployContext)
	done, err := dashboard.Reconcile()
	if !done || err != nil {
		t.Fatalf("Failed to sync Dashboard: %v", err)
	}

	// check service
	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Service not found: %v", err)
	}

	// check endpoint
	route := &routev1.Route{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, route)
	if err != nil {
		t.Fatalf("Route not found: %v", err)
	}

	// check deployment
	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Deployment not found: %v", err)
	}

	sa := &corev1.ServiceAccount{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: DashboardSA, Namespace: "eclipse-che"}, sa)
	if err != nil {
		t.Fatalf("Service account not found: %v", err)
	}

	err = cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleTemplate, Namespace)}, &rbacv1.ClusterRole{})
	if err == nil || !errors.IsNotFound(err) {
		t.Fatalf("ClusterRole is created or failed to check on OpenShift: %v", err)
	}

	err = cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleBindingTemplate, Namespace)}, &rbacv1.ClusterRoleBinding{})
	if err == nil || !errors.IsNotFound(err) {
		t.Fatalf("ClusterRoleBinding is created or failed to check on OpenShift: %v", err)
	}
}

func TestDashboardKubernetes(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Namespace,
			Name:      "eclipse-che",
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	routev1.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)
	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	util.IsOpenShift = false

	dashboard := NewDashboard(deployContext)
	done, err := dashboard.Reconcile()
	if !done || err != nil {
		t.Fatalf("Failed to sync Dashboard: %v", err)
	}

	// check service
	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Service not found: %v", err)
	}

	// check endpoint
	ingress := &networkingv1.Ingress{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, ingress)
	if err != nil {
		t.Fatalf("Ingress not found: %v", err)
	}

	// check deployment
	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Deployment not found: %v", err)
	}

	sa := &corev1.ServiceAccount{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: DashboardSA, Namespace: "eclipse-che"}, sa)
	if err != nil {
		t.Fatalf("Service account not found: %v", err)
	}

	clusterRole := &rbacv1.ClusterRole{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleTemplate, Namespace)}, clusterRole)
	if err != nil {
		t.Fatalf("ClusterRole is not found on K8s: %v", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleBindingTemplate, Namespace)}, clusterRoleBinding)
	if err != nil {
		t.Fatalf("ClusterRoleBinding is not found on k8s: %v", err)
	}
}
