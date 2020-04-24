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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

func SyncRoleToCluster(
	checluster *orgv1.CheCluster,
	name string,
	resources []string,
	verbs []string,
	clusterAPI ClusterAPI) (*rbac.Role, reconcile.Result, error) {

	specRole, err := getSpecRole(checluster, name, resources, verbs, clusterAPI)
	if err != nil {
		return nil, reconcile.Result{}, err
	}

	clusterRole, err := getClusterRole(specRole.Name, specRole.Namespace, clusterAPI.Client)
	if err != nil {
		return nil, reconcile.Result{RequeueAfter: time.Second}, err
	}

	if clusterRole == nil {
		logrus.Infof("Creating a new object: %s, name %s", specRole.Kind, specRole.Name)
		err := clusterAPI.Client.Create(context.TODO(), specRole)
		return nil, reconcile.Result{Requeue: true}, err
	}

	return clusterRole, reconcile.Result{}, nil
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

func getSpecRole(checluster *orgv1.CheCluster, name string, resources []string, verbs []string, clusterAPI ClusterAPI) (*rbac.Role, error) {
	labels := GetLabels(checluster, util.GetValue(checluster.Spec.Server.CheFlavor, DefaultCheFlavor))
	role := &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: resources,
				Verbs:     verbs,
			},
		},
	}

	err := controllerutil.SetControllerReference(checluster, role, clusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return role, nil
}
