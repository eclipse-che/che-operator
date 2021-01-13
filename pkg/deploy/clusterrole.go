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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	
	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var clusterRoleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRole{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbac.PolicyRule{}, "ResourceNames", "NonResourceURLs"),
}

// func SyncTLSRoleToCluster(deployContext *DeployContext) (*rbac.Role, error) {
// 	tlsPolicyRule := []rbac.PolicyRule{
// 		{
// 			APIGroups: []string{
// 				"",
// 			},
// 			Resources: []string{
// 				"secrets",
// 			},
// 			Verbs: []string{
// 				"create",
// 			},
// 		},
// 	}
// 	return SyncRoleToCluster(deployContext, CheTLSJobRoleName, tlsPolicyRule)
// }

// func SyncExecClusterRoleToCluster(deployContext *DeployContext) (*rbac.Role, error) {
// 	execPolicyRule := []rbac.PolicyRule{
// 		{
// 			APIGroups: []string{
// 				"",
// 			},
// 			Resources: []string{
// 				"pods/exec",
// 			},
// 			Verbs: []string{
// 				"*",
// 			},
// 		},
// 	}
// 	return SyncRoleToCluster(deployContext, "exec", execPolicyRule)
// }

// func SyncViewClusterRoleToCluster(deployContext *DeployContext) (*rbac.Role, error) {
// 	viewPolicyRule := []rbac.PolicyRule{
// 		{
// 			APIGroups: []string{
// 				"",
// 			},
// 			Resources: []string{
// 				"pods",
// 			},
// 			Verbs: []string{
// 				"list", "get",
// 			},
// 		},
// 		{
// 			APIGroups: []string{
// 				"metrics.k8s.io",
// 			},
// 			Resources: []string{
// 				"pods",
// 			},
// 			Verbs: []string{
// 				"list", "get", "watch",
// 			},
// 		},
// 	}
// 	return SyncRoleToCluster(deployContext, "view", viewPolicyRule)
// }


// {
// 	APIGroups: []string{"authorization.openshift.io", "rbac.authorization.k8s.io"},
// 	Resources: []string{"roles"},
// 	Verbs: []string{"get", "create"},
// },
// {
// 	APIGroups: []string{"authorization.openshift.io", "rbac.authorization.k8s.io"},
// 	Resources: []string{"rolebindings"},
// 	Verbs: []string{"get", "update", "create"},
// },
// {
// 	APIGroups: []string{"project.openshift.io"},
// 	Resources: []string{"projects"},
// 	Verbs: []string{"get"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"serviceaccounts"},
// 	Verbs: []string{"get", "create", "watch"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"pods/exec"},
// 	Verbs: []string{"create"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"persistentvolumeclaims", "configmaps"},
// 	Verbs: []string{"list"},
// },
// {
// 	APIGroups: []string{"apps"},
// 	Resources: []string{"secrets"},
// 	Verbs: []string{"list"},
// },

// {
// 	APIGroups: []string{""},
// 	Resources: []string{"secrets"},
// 	Verbs: []string{"list", "create", "delete"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"persistentvolumeclaims"},
// 	Verbs: []string{"get", "create", "watch"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"pods"},
// 	Verbs: []string{"get", "create", "list", "watch", "delete"},
// },
// {
// 	APIGroups: []string{"apps"},
// 	Resources: []string{"deployments"},
// 	Verbs: []string{"get", "create", "list", "watch", "patch", "delete"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"services"},
// 	Verbs: []string{"create", "list", "delete"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"configmaps"},
// 	Verbs: []string{"create", "delete"},
// },
// {
// 	APIGroups: []string{"route.openshift.io"},
// 	Resources: []string{"routes"},
// 	Verbs: []string{"list", "create", "delete"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"events"},
// 	Verbs: []string{"watch"},
// },
// {
// 	APIGroups: []string{"apps"},
// 	Resources: []string{"replicasets"},
// 	Verbs: []string{"list", "get", "patch", "delete"},
// },
// {
// 	APIGroups: []string{"extensions"},
// 	Resources: []string{"ingresses"},
// 	Verbs: []string{"list", "create", "watch", "get", "delete"},
// },
// {
// 	APIGroups: []string{""},
// 	Resources: []string{"namespaces"},
// 	Verbs: []string{"get"},
// },

func SyncClusterRoleToCheCluster(
	deployContext *DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (*rbac.ClusterRole, error) {

	specClusterRole, err := getSpecClusterRole(deployContext, name, policyRule)
	if err != nil {
		return nil, err
	}

	clusterRole, err := GetClusterRole(specClusterRole.Name, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterRole == nil {
		logrus.Infof("Creating a new object: %s, name %s", specClusterRole.Kind, specClusterRole.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specClusterRole)
		return nil, err
	}

	diff := cmp.Diff(clusterRole, specClusterRole, clusterRoleDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterRole.Kind, clusterRole.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterRole.Rules = specClusterRole.Rules
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterRole)
		return nil, err
	}

	return clusterRole, nil
}

func GetClusterRole(name string, client runtimeClient.Client) (*rbac.ClusterRole, error) {
	clusterRole := &rbac.ClusterRole{}
	namespacedName := types.NamespacedName{
		Name: name,
	}
	err := client.Get(context.TODO(), namespacedName, clusterRole)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return clusterRole, nil
}

func getSpecClusterRole(deployContext *DeployContext, name string, policyRule []rbac.PolicyRule) (*rbac.ClusterRole, error) {
	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	clusterRole := &rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Labels:    labels,
		},
		Rules: policyRule,
	}

	return clusterRole, nil
}
