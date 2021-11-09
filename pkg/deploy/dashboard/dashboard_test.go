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

package dashboard

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	//given
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
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	//when
	util.IsOpenShift = true
	dashboard := NewDashboard(deployContext)
	done, err := dashboard.Reconcile()
	if !done || err != nil {
		t.Fatalf("Failed to sync Dashboard: %v", err)
	}

	//then
	verifyDashboardServiceExist(t, cli, dashboard)
	verifyDashboardRouteExist(t, cli, dashboard)
	verifyDashboardDeploymentExists(t, cli, dashboard)
	verifyDashboardServiceAccountExists(t, cli)
	verifyClusterRoleDoesNotExist(t, cli)
	verifyClusterRoleBindingDoesNotExist(t, cli)
	verifyFinalizerIsNotSet(t, cheCluster)
}

func TestDashboardKubernetes(t *testing.T) {
	//given
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
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	//when
	util.IsOpenShift = false
	dashboard := NewDashboard(deployContext)
	done, err := dashboard.Reconcile()
	if !done || err != nil {
		t.Fatalf("Failed to sync Dashboard: %v", err)
	}

	//then
	verifyDashboardDeploymentExists(t, cli, dashboard)
	verifyDashboardServiceExist(t, cli, dashboard)
	verifyDashboardIngressExist(t, cli, dashboard)
	verifyDashboardServiceAccountExists(t, cli)
	verifyDashboardClusterRoleExists(t, cli)
	verifyDashboardClusterRoleBindingExists(t, cli)
	verifyFinalizerIsSet(t, cheCluster)
}

func TestDashboardClusterRBACFinalizerOnKubernetes(t *testing.T) {
	//given
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
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	//when
	util.IsOpenShift = false
	dashboard := NewDashboard(deployContext)
	done, err := dashboard.Reconcile()
	if !done || err != nil {
		t.Fatalf("Failed to sync Dashboard: %v", err)
	}
	verifyDashboardClusterRoleExists(t, cli)
	verifyDashboardClusterRoleBindingExists(t, cli)
	verifyFinalizerIsSet(t, cheCluster)
	done, err = dashboard.Finalize()
	if err != nil {
		t.Fatalf("Can't finalize dashboard %v", err)
	}

	//then
	verifyClusterRoleDoesNotExist(t, cli)
	verifyClusterRoleBindingDoesNotExist(t, cli)
	verifyFinalizerIsNotSet(t, cheCluster)
}

func verifyFinalizerIsSet(t *testing.T, cheCluster *orgv1.CheCluster) {
	if !hasFinalizer(ClusterPermissionsDashboardFinalizer, cheCluster) {
		t.Fatal("CheCluster did not get Dashboard Cluster Permissions finalizer on Kubernetes")
	}
}

func verifyDashboardClusterRoleBindingExists(t *testing.T, cli client.Client) {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleBindingTemplate, Namespace)}, clusterRoleBinding)
	if err != nil {
		t.Fatalf("ClusterRoleBinding is not found on k8s: %v", err)
	}
}

func verifyDashboardClusterRoleExists(t *testing.T, cli client.Client) {
	clusterRole := &rbacv1.ClusterRole{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleTemplate, Namespace)}, clusterRole)
	if err != nil {
		t.Fatalf("ClusterRole is not found on K8s: %v", err)
	}
}

func verifyDashboardIngressExist(t *testing.T, cli client.Client, dashboard *Dashboard) {
	ingress := &networkingv1.Ingress{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, ingress)
	if err != nil {
		t.Fatalf("Ingress not found: %v", err)
	}
}

func verifyFinalizerIsNotSet(t *testing.T, cheCluster *orgv1.CheCluster) {
	if hasFinalizer(ClusterPermissionsDashboardFinalizer, cheCluster) {
		t.Fatal("CheCluster got Dashboard Cluster Permissions finalizer but not expected")
	}
}

func verifyClusterRoleBindingDoesNotExist(t *testing.T, cli client.Client) {
	err := cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleBindingTemplate, Namespace)}, &rbacv1.ClusterRoleBinding{})
	if err == nil || !errors.IsNotFound(err) {
		t.Fatalf("ClusterRoleBinding is created or failed to check on OpenShift: %v", err)
	}
}

func verifyClusterRoleDoesNotExist(t *testing.T, cli client.Client) {
	err := cli.Get(context.TODO(), types.NamespacedName{Name: fmt.Sprintf(DashboardSAClusterRoleTemplate, Namespace)}, &rbacv1.ClusterRole{})
	if err == nil || !errors.IsNotFound(err) {
		t.Fatalf("ClusterRole is created or failed to check on OpenShift: %v", err)
	}
}

func verifyDashboardServiceAccountExists(t *testing.T, cli client.Client) {
	sa := &corev1.ServiceAccount{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: DashboardSA, Namespace: "eclipse-che"}, sa)
	if err != nil {
		t.Fatalf("Service account not found: %v", err)
	}
}

func verifyDashboardDeploymentExists(t *testing.T, cli client.Client, dashboard *Dashboard) {
	deployment := &appsv1.Deployment{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Deployment not found: %v", err)
	}
}

func verifyDashboardRouteExist(t *testing.T, cli client.Client, dashboard *Dashboard) {
	route := &routev1.Route{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, route)
	if err != nil {
		t.Fatalf("Route not found: %v", err)
	}
}

func verifyDashboardServiceExist(t *testing.T, cli client.Client, dashboard *Dashboard) {
	service := &corev1.Service{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: dashboard.component, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Service not found: %v", err)
	}
}

func hasFinalizer(name string, cheCluster *orgv1.CheCluster) bool {
	for _, finalizer := range cheCluster.ObjectMeta.Finalizers {
		if finalizer == name {
			return true
		}
	}
	return false
}
