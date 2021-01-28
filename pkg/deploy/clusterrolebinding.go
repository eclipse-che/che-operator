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

var crbDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRoleBinding{}, "TypeMeta", "ObjectMeta"),
}

func SyncClusterRoleBindingToCluster(
	deployContext *DeployContext,
	name string,
	serviceAccountName string,
	clusterRoleName string) (*rbac.ClusterRoleBinding, error) {

	specCRB, err := getSpecClusterRoleBinding(deployContext, name, serviceAccountName, clusterRoleName)
	if err != nil {
		return nil, err
	}

	clusterRB, err := getClusterRoleBiding(specCRB.Name, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterRB == nil {
		logrus.Infof("Creating a new object: %s, name %s", specCRB.Kind, specCRB.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specCRB)
		return nil, err
	}

	diff := cmp.Diff(clusterRB, specCRB, crbDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterRB.Kind, clusterRB.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterRB.Subjects = specCRB.Subjects
		clusterRB.RoleRef = specCRB.RoleRef
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterRB)
		return clusterRB, err
	}

	return clusterRB, nil
}

func getClusterRoleBiding(name string, client runtimeClient.Client) (*rbac.ClusterRoleBinding, error) {
	clusterRoleBinding := &rbac.ClusterRoleBinding{}
	crbName := types.NamespacedName{Name: name}
	err := client.Get(context.TODO(), crbName, clusterRoleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return clusterRoleBinding, nil
}

func getSpecClusterRoleBinding(
	deployContext *DeployContext,
	name string,
	serviceAccountName string,
	roleName string) (*rbac.ClusterRoleBinding, error) {

	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	clusterRoleBinding := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
			Annotations: map[string]string{
				CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: deployContext.CheCluster.Namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     roleName,
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
		},
	}

	return clusterRoleBinding, nil
}
