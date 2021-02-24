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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func CreateNamespace(deployContext *DeployContext, name string) (bool, error) {
	namespace, err := GetNamespace(deployContext, name)
	if err != nil {
		return false, err
	}

	if namespace == nil {
		namespaceSpec := GetNamespaceSpec(deployContext, name)
		logrus.Infof("Creating a new object: %s, name %s", namespaceSpec.Kind, namespaceSpec.Name)
		err = deployContext.ClusterAPI.NonCachedClient.Create(context.TODO(), namespaceSpec)
		if !errors.IsAlreadyExists(err) {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

func GetNamespace(deployContext *DeployContext, name string) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	namespacedName := types.NamespacedName{
		Name: name,
	}

	err := deployContext.ClusterAPI.NonCachedClient.Get(context.TODO(), namespacedName, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return namespace, nil
}

func GetNamespaceSpec(deployContext *DeployContext, name string) *corev1.Namespace {
	namespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.NamespaceSpec{},
	}

	return namespace
}
