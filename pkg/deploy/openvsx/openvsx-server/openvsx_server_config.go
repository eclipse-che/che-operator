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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *OpenVSXServerReconciler) syncConfigMap(cheCtx *chetypes.CheContext) error {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXServerComponentName),
			Annotations: map[string]string{
				constants.OpenVSXServerOriginalApplicationConfig: applicationConfig,
			},
		},
		Data: map[string]string{
			"application.yml": applicationConfig,
		},
	}

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, cm, cheCtx.ClusterAPI.Scheme); err != nil {
		return err
	}

	if err := cheCtx.ClusterAPI.ClientWrapper.CreateIfNotExists(cheCtx.Context, cm); err != nil {
		return err
	}

	return r.syncOriginalApplicationConfigAnnotation(cheCtx)
}

// syncOriginalApplicationConfigAnnotation keeps the original-config annotation up to date
// on existing ConfigMaps (e.g. after an operator upgrade that changes application.yml).
func (r *OpenVSXServerReconciler) syncOriginalApplicationConfigAnnotation(cheCtx *chetypes.CheContext) error {
	cm := &corev1.ConfigMap{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: cheCtx.CheCluster.Namespace,
		},
		cm,
	)
	if err != nil || !exists {
		return err
	}

	if cm.Annotations[constants.OpenVSXServerOriginalApplicationConfig] == applicationConfig {
		return nil
	}

	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}
	cm.Annotations[constants.OpenVSXServerOriginalApplicationConfig] = applicationConfig

	if err := cheCtx.ClusterAPI.Client.Update(cheCtx.Context, cm); err != nil {
		return fmt.Errorf("failed to update ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
	}

	return nil
}

func (r *OpenVSXServerReconciler) getConfigRevision(cheCtx *chetypes.CheContext) (string, error) {
	cm := &corev1.ConfigMap{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: cheCtx.CheCluster.Namespace,
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
