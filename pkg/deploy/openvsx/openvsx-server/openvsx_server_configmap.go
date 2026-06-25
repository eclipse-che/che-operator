//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package openvsx_server

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *OpenVSXServerReconciler) syncConfigMap(ctx *chetypes.DeployContext) error {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXServerComponentName),
		},
		Data: map[string]string{
			"application.yml": applicationConfig,
		},
	}

	return ctx.ClusterAPI.ClientWrapper.CreateIfNotExists(context.TODO(), cm)
}

func (r *OpenVSXServerReconciler) getConfigRevision(ctx *chetypes.DeployContext) (string, error) {
	cm := &corev1.ConfigMap{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: ctx.CheCluster.Namespace,
		},
		cm,
	)
	if err != nil {
		return "", err
	}
	if exists {
		return cm.ResourceVersion, nil
	}

	return "", nil
}
