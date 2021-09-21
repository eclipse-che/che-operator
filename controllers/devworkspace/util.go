//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devworkspace

import (
	chev2alpha1 "github.com/eclipse-che/che-operator/api/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

type DevworkspaceState int

const (
	DevworkspaceStateNotPresent DevworkspaceState = 0
	DevworkspaceStateDisabled   DevworkspaceState = 1
	DevworkspaceStateEnabled    DevworkspaceState = 2
)

func GetDevworkspaceState(scheme *runtime.Scheme, cr *chev2alpha1.CheCluster) DevworkspaceState {
	if !scheme.IsGroupRegistered("controller.devfile.io") {
		return DevworkspaceStateNotPresent
	}

	if !cr.Spec.IsEnabled() {
		return DevworkspaceStateDisabled
	}

	return DevworkspaceStateEnabled
}
