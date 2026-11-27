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
	"maps"
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/provision/syncher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

var Service = &serviceHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(corev1.Service{}, "TypeMeta"),
		//var Service = cmp.Options{
		//	cmpopts.IgnoreFields(corev1.Service{}, "TypeMeta", "ObjectMeta", "Status"),
		cmp.Comparer(func(x, y corev1.ServiceSpec) bool {
			return maps.Equal(x.Selector, y.Selector) && reflect.DeepEqual(x.Ports, y.Ports)
		}),
		//}

		//cmp.Comparer(func(x, y corev1.ServiceSpec) bool {
		//	return cmp.Equal(x.Ports, y.Ports, cmpopts.IgnoreFields(corev1.ServicePort{}, "NodePort")) &&
		//		cmp.Equal(x.Selector, y.Selector)
		//}),
	},
}

type serviceHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h *serviceHandler) Sync(cheCtx *chetypes.CheContext, service *corev1.Service) error {
	return syncher.SyncObject(
		cheCtx,
		service,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}
