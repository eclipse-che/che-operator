//
// Copyright (c) 2019-2024 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package utils

import (
	corev1 "k8s.io/api/core/v1"
)

// IndexVolumeMount returns the index of the volume mount with the given name, or -1 if not found.
func IndexVolumeMount(name string, mounts []corev1.VolumeMount) int {
	for i, m := range mounts {
		if m.Name == name {
			return i
		}
	}
	return -1
}
