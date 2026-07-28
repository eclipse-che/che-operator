//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package usernamespace

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var allUserNsNetworkPolicyNames = []string{
	"allow-from-eclipse-che",
	"allow-from-same-namespace",
	"allow-from-devworkspace-operator",
	"allow-from-openshift-monitoring",
	"allow-from-openshift-ingress",
	"allow-all-ingress",
}

func TestNetworkPoliciesCreatedWhenEnabledOnOpenShift(t *testing.T) {
	cheCluster, userNamespace, userProject := buildNetworkPolicyTestObjects()
	_, cl, r := setup(infrastructure.OpenShiftV4, cheCluster, userNamespace, userProject)

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	npList := &networkingv1.NetworkPolicyList{}
	err = cl.List(context.TODO(), npList, &client.ListOptions{Namespace: "user-project"})
	assert.NoError(t, err)
	assert.Equal(t, len(allUserNsNetworkPolicyNames), len(npList.Items))

	for _, name := range allUserNsNetworkPolicyNames {
		np := &networkingv1.NetworkPolicy{}
		err := cl.Get(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "user-project"},
			np,
		)
		assert.NoError(t, err)
	}
}

func TestNetworkPoliciesDeletedWhenDisabled(t *testing.T) {
	cheCluster, userNamespace, userProject := buildNetworkPolicyTestObjects()
	_, cl, r := setup(infrastructure.OpenShiftV4, cheCluster, userNamespace, userProject)

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	cheCluster.Spec.Networking.NetworkPolicy.Enabled = ptr.To(false)
	err = cl.Update(context.TODO(), cheCluster)
	assert.NoError(t, err)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	for _, name := range allUserNsNetworkPolicyNames {
		np := &networkingv1.NetworkPolicy{}
		err := cl.Get(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "user-project"},
			np,
		)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestNetworkPoliciesIdempotent(t *testing.T) {
	cheCluster, userNamespace, userProject := buildNetworkPolicyTestObjects()
	_, cl, r := setup(infrastructure.OpenShiftV4, cheCluster, userNamespace, userProject)

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	for _, name := range allUserNsNetworkPolicyNames {
		np := &networkingv1.NetworkPolicy{}
		err := cl.Get(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "user-project"},
			np,
		)
		assert.NoError(t, err)
	}
}

func TestNetworkPoliciesDeletedOnlyOwnedPolicies(t *testing.T) {
	cheCluster, userNamespace, userProject := buildNetworkPolicyTestObjects()

	unownedNetworkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unowned-policy",
			Namespace: "user-project",
		},
	}

	_, cl, r := setup(
		infrastructure.OpenShiftV4,
		cheCluster,
		userNamespace,
		userProject,
		unownedNetworkPolicy,
	)

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	cheCluster.Spec.Networking.NetworkPolicy.Enabled = ptr.To(false)
	err = cl.Update(context.TODO(), cheCluster)
	assert.NoError(t, err)

	_, err = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	for _, name := range allUserNsNetworkPolicyNames {
		np := &networkingv1.NetworkPolicy{}
		err := cl.Get(
			context.TODO(),
			types.NamespacedName{Name: name, Namespace: "user-project"},
			np,
		)
		assert.Error(t, err)
		assert.True(t, errors.IsNotFound(err))
	}

	np := &networkingv1.NetworkPolicy{}
	err = cl.Get(
		context.TODO(),
		types.NamespacedName{Name: "unowned-policy", Namespace: "user-project"},
		np,
	)
	assert.NoError(t, err)
}

func buildNetworkPolicyTestObjects() (*chev2.CheCluster, client.Object, client.Object) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				NetworkPolicy: &chev2.NetworkPolicy{
					Enabled: ptr.To(true),
				},
			},
		},
	}

	userNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-project",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:           constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:        "workspaces-namespace",
				constants.WorkspaceNamespaceOwnerUidLabelKey: "some-uid",
			},
		},
	}

	userProject := &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-project",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:           constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:        "workspaces-namespace",
				constants.WorkspaceNamespaceOwnerUidLabelKey: "some-uid",
			},
		},
	}

	return cheCluster, userNamespace, userProject
}
