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

	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncRoleBindingToCluster(
	deployContext *DeployContext,
	name string,
	serviceAccountName string,
	roleName string,
	roleKind string) (*rbac.RoleBinding, error) {

	specRB, err := getSpecRoleBinding(deployContext, name, serviceAccountName, roleName, roleKind)
	if err != nil {
		return nil, err
	}

	roleBinding, err := getRoleBiding(specRB.Name, specRB.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if roleBinding == nil {
		logrus.Infof("Creating a new object: %s, name %s", specRB.Kind, specRB.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specRB)
		return nil, err
	}

	return roleBinding, nil
}

func getRoleBiding(name string, namespace string, client runtimeClient.Client) (*rbac.RoleBinding, error) {
	roleBinding := &rbac.RoleBinding{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, roleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return roleBinding, nil
}

func getSpecRoleBinding(
	deployContext *DeployContext,
	name string,
	serviceAccountName string,
	roleName string,
	roleKind string) (*rbac.RoleBinding, error) {

	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	roleBinding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
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
			Kind:     roleKind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	err := controllerutil.SetControllerReference(deployContext.CheCluster, roleBinding, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return roleBinding, nil
}
