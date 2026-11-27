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

package resources

import corev1 "k8s.io/api/core/v1"

var VolumeSource = &volumeSourceHandler{}

type volumeSourceHandler struct{}

func (h *volumeSourceHandler) FromConfigMap(name string, keys ...string) corev1.VolumeSource {
	return corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: name,
			},
			DefaultMode: new(corev1.ConfigMapVolumeSourceDefaultMode),
			Items:       keyToPaths(keys),
		},
	}
}

func (h *volumeSourceHandler) FromSecret(name string, keys ...string) corev1.VolumeSource {
	return corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  name,
			DefaultMode: new(corev1.SecretVolumeSourceDefaultMode),
			Items:       keyToPaths(keys),
		},
	}
}

func keyToPaths(keys []string) []corev1.KeyToPath {
	if len(keys) == 0 {
		return nil
	}

	items := make([]corev1.KeyToPath, 0, len(keys))
	for _, key := range keys {
		items = append(items, corev1.KeyToPath{Key: key, Path: key})
	}

	return items
}
