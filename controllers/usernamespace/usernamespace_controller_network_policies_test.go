//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	networkingv1 "k8s.io/api/networking/v1"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	projectv1 "github.com/openshift/api/project/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestNetworkPoliciesCreatedWhenEnabledOnOpenShift(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				Networking: &chev2.DevEnvironmentNetworking{
					NetworkPolicies: &chev2.NetworkPolicies{
						Enabled: true,
					},
				},
			},
		},
	}

	userNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-project",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:             constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:          "workspaces-namespace",
				namespacecache.WorkspaceNamespaceOwnerUidLabel: "some-uid",
			},
		},
	}

	userProject := &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-project",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:             constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:          "workspaces-namespace",
				namespacecache.WorkspaceNamespaceOwnerUidLabel: "some-uid",
			},
		},
	}

	_, cl, r := setup(infrastructure.OpenShiftV4, cheCluster, userProject, userNamespace)

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	policyNames := []string{
		"allow-from-" + defaults.GetCheFlavor(),
		"allow-from-same-namespace",
		"allow-from-operators",
		"allow-from-openshift-monitoring",
		"allow-from-openshift-ingress",
	}
	for _, name := range policyNames {
		networkPolicy := &networkingv1.NetworkPolicy{}
		err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: "user-project"}, networkPolicy)

		assert.NoError(t, err)
		assert.Equal(t, constants.CheEclipseOrg, networkPolicy.Labels[constants.KubernetesPartOfLabelKey])
	}

	networkPolicyList := &networkingv1.NetworkPolicyList{}
	err = cl.List(context.TODO(), networkPolicyList)
	assert.NoError(t, err)
	assert.Len(t, networkPolicyList.Items, 5)
}

func TestNetworkPoliciesCreatedWhenEnabledOnKubernetes(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				Networking: &chev2.DevEnvironmentNetworking{
					NetworkPolicies: &chev2.NetworkPolicies{
						Enabled: true,
					},
				},
			},
		},
	}

	userNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-project",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:             constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:          "workspaces-namespace",
				namespacecache.WorkspaceNamespaceOwnerUidLabel: "some-uid",
			},
		},
	}

	_, cl, r := setup(infrastructure.Kubernetes, cheCluster, userNamespace)

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	policyNames := []string{
		"allow-from-" + defaults.GetCheFlavor(),
		"allow-from-same-namespace",
		"allow-from-operators",
	}
	for _, name := range policyNames {
		networkPolicy := &networkingv1.NetworkPolicy{}
		err := cl.Get(context.TODO(), client.ObjectKey{Name: name, Namespace: "user-project"}, networkPolicy)

		assert.NoError(t, err)
		assert.Equal(t, constants.CheEclipseOrg, networkPolicy.Labels[constants.KubernetesPartOfLabelKey])
	}

	networkPolicyList := &networkingv1.NetworkPolicyList{}
	err = cl.List(context.TODO(), networkPolicyList)
	assert.NoError(t, err)
	assert.Len(t, networkPolicyList.Items, 3)
}

func TestNetworkPoliciesDeletedWhenDisabled(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				Networking: &chev2.DevEnvironmentNetworking{
					NetworkPolicies: &chev2.NetworkPolicies{
						Enabled: false,
					},
				},
			},
		},
	}

	allowFromCheNetworkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-from-" + defaults.GetCheFlavor(),
			Namespace: "user-project",
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
	}

	anotherAllowFromCheNetworkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-allow-from-" + defaults.GetCheFlavor(),
			Namespace: "user-project",
		},
	}

	userNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-project",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:             constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:          "workspaces-namespace",
				namespacecache.WorkspaceNamespaceOwnerUidLabel: "some-uid",
			},
		},
	}

	userProject := &projectv1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name: "user-project",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:             constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:          "workspaces-namespace",
				namespacecache.WorkspaceNamespaceOwnerUidLabel: "some-uid",
			},
		},
	}

	_, cl, r := setup(
		infrastructure.OpenShiftV4,
		cheCluster,
		userProject,
		userNamespace,
		allowFromCheNetworkPolicy,
		anotherAllowFromCheNetworkPolicy,
	)

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "user-project"}})
	assert.NoError(t, err)

	networkPolicy := &networkingv1.NetworkPolicy{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: "allow-from-" + defaults.GetCheFlavor(), Namespace: "user-project"}, networkPolicy)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	err = cl.Get(context.TODO(), client.ObjectKey{Name: "another-allow-from-" + defaults.GetCheFlavor(), Namespace: "user-project"}, networkPolicy)
	assert.NoError(t, err)
}
