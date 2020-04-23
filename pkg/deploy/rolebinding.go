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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type RoleBindingProvisioningStatus struct {
	ProvisioningStatus
}

func SyncRoleBindingToCluster(
	checluster *orgv1.CheCluster,
	name string,
	serviceAccountName string,
	roleName string,
	roleKind string,
	clusterAPI ClusterAPI) RoleBindingProvisioningStatus {

	specRB, err := getSpecRoleBinding(checluster, name, serviceAccountName, roleName, roleKind, clusterAPI)
	if err != nil {
		return RoleBindingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	clusterRB, err := getClusterRoleBiding(specRB.Name, specRB.Namespace, clusterAPI.Client)
	if err != nil {
		return RoleBindingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterRB == nil {
		logrus.Infof("Creating a new object: %s, name %s", specRB.Kind, specRB.Name)
		err := clusterAPI.Client.Create(context.TODO(), specRB)
		return RoleBindingProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	return RoleBindingProvisioningStatus{ProvisioningStatus: ProvisioningStatus{Continue: true}}
}

func getClusterRoleBiding(name string, namespace string, client runtimeClient.Client) (*rbac.RoleBinding, error) {
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
	checluster *orgv1.CheCluster,
	name string,
	serviceAccountName string,
	roleName string,
	roleKind string,
	clusterAPI ClusterAPI) (*rbac.RoleBinding, error) {

	labels := GetLabels(checluster, util.GetValue(checluster.Spec.Server.CheFlavor, DefaultCheFlavor))
	roleBinding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: checluster.Namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     roleName,
			Kind:     roleKind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	err := controllerutil.SetControllerReference(checluster, roleBinding, clusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return roleBinding, nil
}
