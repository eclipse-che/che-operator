//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var roleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbac.PolicyRule{}, "Verbs", "APIGroups", "Resources", "ResourceNames", "NonResourceURLs"),
	cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	}),
}

func SyncTLSRoleToCluster(deployContext *DeployContext) (*rbac.Role, error) {
	tlsPolicyRule := []rbac.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
			},
			Verbs:     []string{
				"create",
			},
		},
	}
	return SyncRoleToCluster(deployContext, CheTLSJobRoleName, tlsPolicyRule)	
}

func SyncExecRoleToCluster(deployContext *DeployContext) (*rbac.Role, error) {
	execPolicyRule := []rbac.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods/exec",
			},
			Verbs:     []string{
				"*",
			},
		},
	}
	return SyncRoleToCluster(deployContext, "exec", execPolicyRule)	
}

func SyncViewRoleToCluster(deployContext *DeployContext) (*rbac.Role, error) {
	viewPolicyRule := []rbac.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs:     []string{
				"list", "get",
			},
		},
		{
			APIGroups: []string{
				"metrics.k8s.io",
			},
			Resources: []string{
				"pods",
			},
			Verbs:     []string{
				"list", "get", "watch",
			},
		},
	}
	return SyncRoleToCluster(deployContext, "view", viewPolicyRule)	
}

func SyncRoleToCluster(
	deployContext *DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (*rbac.Role, error) {

	specRole, err := getSpecRole(deployContext, name, policyRule)
	if err != nil {
		return nil, err
	}

	clusterRole, err := getClusterRole(specRole.Name, specRole.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterRole == nil {
		logrus.Infof("Creating a new object: %s, name %s", specRole.Kind, specRole.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specRole)
		return nil, err
	}
	
	diff := cmp.Diff(clusterRole, specRole, roleDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterRole.Kind, clusterRole.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterRole.Rules = specRole.Rules
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterRole)
		return nil, err
	}

	return clusterRole, nil
}

func getClusterRole(name string, namespace string, client runtimeClient.Client) (*rbac.Role, error) {
	role := &rbac.Role{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, role)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return role, nil
}

func getSpecRole(deployContext *DeployContext, name string, policyRule []rbac.PolicyRule) (*rbac.Role, error) {
	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	role := &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Rules: policyRule,
	}

	err := controllerutil.SetControllerReference(deployContext.CheCluster, role, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return role, nil
}
