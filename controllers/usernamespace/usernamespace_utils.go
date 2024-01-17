//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package usernamespace

import (
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type eventRule struct {
	check      func(metav1.Object) bool
	namespaces func(metav1.Object) []string
}

func asReconcileRequestsForNamespaces(obj metav1.Object, rules []eventRule) []reconcile.Request {
	for _, r := range rules {
		if r.check(obj) {
			nss := r.namespaces(obj)
			ret := make([]reconcile.Request, len(nss))
			for i, n := range nss {
				ret[i] = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: n,
					},
				}
			}

			return ret
		}
	}

	return []reconcile.Request{}
}

func addDefaultLabels(labels map[string]string) {
	utils.AddMap(labels, deploy.GetLabels(constants.WorkspacesConfig))
}
