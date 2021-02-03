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
)

var clusterRoleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRole{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbac.PolicyRule{}, "ResourceNames", "NonResourceURLs"),
}

func SyncClusterRoleToCheCluster(
	deployContext *DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (bool, error) {

	specClusterRole, err := getSpecClusterRole(deployContext, name, policyRule)
	if err != nil {
		return false, err
	}

	clusterRole, err := GetClusterRole(specClusterRole.Name, deployContext.ClusterAPI.Client)
	if err != nil {
		return false, err
	}

	if clusterRole == nil {
		logrus.Infof("Creating a new object: %s, name %s", specClusterRole.Kind, specClusterRole.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specClusterRole)
		return false, err
	}

	diff := cmp.Diff(clusterRole, specClusterRole, clusterRoleDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterRole.Kind, clusterRole.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterRole.Rules = specClusterRole.Rules
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterRole)
		return false, err
	}

	return true, nil
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
			Name:   name,
			Labels: labels,
		},
		Rules: policyRule,
	}

	return clusterRole, nil
}

func DeleteClusterRole(clusterRoleName string, client runtimeClient.Client) error {
	clusterRole := &rbac.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
	}
	err := client.Delete(context.TODO(), clusterRole)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
