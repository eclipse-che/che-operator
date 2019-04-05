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
	"github.com/sirupsen/logrus"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"path/filepath"
)

func deserializeOperatorDeployment() (operatorDeployment *appsv1.Deployment, err error) {
	fileLocation, err := filepath.Abs("deploy/operator-local.yaml")
	if err != nil {
		logrus.Fatalf("Failed to locate operator deployment yaml, %s", err)
	}
	file, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		logrus.Errorf("Failed to locate operator deployment yaml, %s", err)
	}
	deployment := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(deployment), nil, nil)
	if err != nil {
		logrus.Errorf("Failed to deserialize yaml %s", err)
		return nil, err
	}
	operatorDeployment = object.(*appsv1.Deployment)
	return operatorDeployment, nil
}

func deserializeOperatorServiceAccount() (operatorServiceAccount *corev1.ServiceAccount, err error) {
	fileLocation, err := filepath.Abs("deploy/service_account.yaml")
	if err != nil {
		logrus.Fatalf("Failed to locate operator service account yaml, %s", err)
	}
	file, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		logrus.Errorf("Failed to locate operator service account yaml, %s", err)
	}
	sa := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(sa), nil, nil)
	if err != nil {
		logrus.Errorf("Failed to deserialize yaml %s", err)
		return nil, err
	}
	operatorServiceAccount = object.(*corev1.ServiceAccount)
	return operatorServiceAccount, nil
}

func deserializeOperatorRole() (operatorServiceAccountRole *rbac.Role, err error) {
	fileLocation, err := filepath.Abs("deploy/role.yaml")
	if err != nil {
		logrus.Fatalf("Failed to locate operator service account role yaml, %s", err)
	}
	file, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		logrus.Errorf("Failed to locate operator service account role yaml, %s", err)
	}
	role := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(role), nil, nil)
	if err != nil {
		logrus.Errorf("Failed to deserialize yaml %s", err)
		return nil, err
	}
	operatorServiceAccountRole = object.(*rbac.Role)
	return operatorServiceAccountRole, nil
}

func deserializeOperatorClusterRole() (operatorServiceAccountClusterRole *rbac.ClusterRole, err error) {
	fileLocation, err := filepath.Abs("deploy/cluster_role.yaml")
	if err != nil {
		logrus.Fatalf("Failed to locate operator service account cluster role yaml, %s", err)
	}
	file, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		logrus.Errorf("Failed to locate operator service account cluster role yaml, %s", err)
	}
	role := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(role), nil, nil)
	if err != nil {
		logrus.Errorf("Failed to deserialize yaml %s", err)
		return nil, err
	}
	operatorServiceAccountClusterRole = object.(*rbac.ClusterRole)
	return operatorServiceAccountClusterRole, nil
}

func deserializeOperatorRoleBinding() (operatorServiceAccountRoleBinding *rbac.RoleBinding, err error) {
	fileLocation, err := filepath.Abs("deploy/role_binding.yaml")
	if err != nil {
		logrus.Fatalf("Failed to locate operator service account role binding yaml, %s", err)
	}
	file, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		logrus.Errorf("Failed to locate operator service account role binding yaml, %s", err)
	}
	roleBinding := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(roleBinding), nil, nil)
	if err != nil {
		logrus.Errorf("Failed to deserialize yaml %s", err)
		return nil, err
	}
	operatorServiceAccountRoleBinding = object.(*rbac.RoleBinding)
	return operatorServiceAccountRoleBinding, nil
}


func deserializeOperatorClusterRoleBinding() (operatorServiceAccountClusterRoleBinding *rbac.ClusterRoleBinding, err error) {
	fileLocation, err := filepath.Abs("deploy/cluster_role_binding.yaml")
	if err != nil {
		logrus.Fatalf("Failed to locate operator service account role binding yaml, %s", err)
	}
	file, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		logrus.Errorf("Failed to locate operator service account role binding yaml, %s", err)
	}
	roleBinding := string(file)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	object, _, err := decode([]byte(roleBinding), nil, nil)
	if err != nil {
		logrus.Errorf("Failed to deserialize yaml %s", err)
		return nil, err
	}
	operatorServiceAccountClusterRoleBinding = object.(*rbac.ClusterRoleBinding)
	return operatorServiceAccountClusterRoleBinding, nil
}