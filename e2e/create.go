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
package main

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createOperatorServiceAccount(operatorServiceAccount *corev1.ServiceAccount) (err error) {

	operatorServiceAccount, err = client.clientset.CoreV1().ServiceAccounts(namespace).Create(operatorServiceAccount)
	if err != nil {
		logrus.Fatalf("Failed to create service account %s: %s", operatorServiceAccount.Name, err)
		return err
	}
	return nil

}

func createOperatorServiceAccountRole(operatorServiceAccountRole *rbac.Role) (err error) {

	operatorServiceAccountRole, err = client.clientset.RbacV1().Roles(namespace).Create(operatorServiceAccountRole)
	if err != nil {
		logrus.Fatalf("Failed to create role %s: %s", operatorServiceAccountRole.Name, err)
		return err
	}
	return nil

}

func createOperatorServiceAccountClusterRole(operatorServiceAccountClusterRole *rbac.ClusterRole) (err error) {

	operatorServiceAccountClusterRole, err = client.clientset.RbacV1().ClusterRoles().Create(operatorServiceAccountClusterRole)
	if err != nil && ! errors.IsAlreadyExists(err) {
		logrus.Fatalf("Failed to create role %s: %s", operatorServiceAccountClusterRole.Name, err)
		return err
	}
	return nil

}

func createOperatorServiceAccountRoleBinding(operatorServiceAccountRoleBinding *rbac.RoleBinding) (err error) {

	operatorServiceAccountRoleBinding, err = client.clientset.RbacV1().RoleBindings(namespace).Create(operatorServiceAccountRoleBinding)
	if err != nil {
		logrus.Fatalf("Failed to create role %s: %s", operatorServiceAccountRoleBinding.Name, err)
		return err
	}
	return nil

}

func createOperatorServiceAccountClusterRoleBinding(operatorServiceAccountClusterRoleBinding *rbac.ClusterRoleBinding) (err error) {

	operatorServiceAccountClusterRoleBinding, err = client.clientset.RbacV1().ClusterRoleBindings().Create(operatorServiceAccountClusterRoleBinding)
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Fatalf("Failed to create role %s: %s", operatorServiceAccountClusterRoleBinding.Name, err)
		return err
	}
	return nil

}

func deployOperator(deployment *appsv1.Deployment) (err error) {

	deployment, err = client.clientset.AppsV1().Deployments(namespace).Create(deployment)
	if err != nil {
		logrus.Fatalf("Failed to create deployment %s: %s", deployment.Name, err)
		return err
	}
	return nil

}

func newNamespace() (ns *corev1.Namespace){

	return &corev1.Namespace{

		TypeMeta: metav1.TypeMeta{
			Kind: "Namespace",
			APIVersion: corev1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:namespace,
		},

	}
}

func createNamespace(ns *corev1.Namespace) (err error) {

	ns, err = client.clientset.CoreV1().Namespaces().Create(ns)
	if err != nil {
		logrus.Fatalf("Failed to create namespace %s: %s", ns.Name, err)
		return err
	}
	return nil

}

func newCheCluster() (cr *orgv1.CheCluster) {
	cr = &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: kind,
		},
		Spec:orgv1.CheClusterSpec{
			Server:orgv1.CheClusterSpecServer{
				SelfSignedCert: true,
			},
		},
	}
	return cr
}

func createCR() (err error) {
	result := orgv1.CheCluster{}
	cheCluster := newCheCluster()
	err = clientSet.restClient.
		Post().
		Namespace(namespace).
		Resource(kind).
		Name(crName).
		Body(cheCluster).
		Do().
		Into(&result)
	if err != nil {
		return err
	}
	return nil
}
