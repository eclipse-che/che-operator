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

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/provision/syncher"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var LimitRange = &limitRangeHandler{}

type limitRangeHandler struct{}

func (h *limitRangeHandler) List(cheCtx *chetypes.CheContext, namespace string) ([]corev1.LimitRange, error) {
	listOptions := &client.ListOptions{Namespace: namespace}

	limitRanges := &corev1.LimitRangeList{}
	if err := syncher.List(cheCtx, limitRanges, listOptions); err != nil {
		return nil, err
	}

	return limitRanges.Items, nil
}
