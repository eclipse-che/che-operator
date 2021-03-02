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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestReconcileDevWorkspace(t *testing.T) {
	scheme := scheme.Scheme
	orgv1.SchemeBuilder.AddToScheme(scheme)
	scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})

	cli := fake.NewFakeClientWithScheme(scheme)

	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
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
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme,
		},
	}

	util.IsOpenShift4 = true
	done, err := ReconcileDevWorkspace(deployContext)

	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	if !done {
		t.Fatalf("Dev Workspace operator has not been provisioned")
	}
}

func TestReconcileDevWorkspaceShouldThrowErrorIfWebTerminalSubscriptionExists(t *testing.T) {
	subscription := &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebTerminalOperatorSubscriptionName,
			Namespace: WebTerminalOperatorNamespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{},
	}

	webhook := admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: DevWorkspaceWebhookName,
		},
	}

	scheme := scheme.Scheme
	orgv1.SchemeBuilder.AddToScheme(scheme)
	scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	scheme.AddKnownTypes(admissionregistrationv1.SchemeGroupVersion, &admissionregistrationv1.MutatingWebhookConfiguration{})

	cli := fake.NewFakeClientWithScheme(scheme, subscription, &webhook)

	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
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
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme,
		},
	}

	util.IsOpenShift4 = true
	_, err := ReconcileDevWorkspace(deployContext)
	if err == nil || err.Error() != "A non matching version of the Dev Workspace operator is already installed" {
		t.Fatalf("Error should be thrown")
	}
}
