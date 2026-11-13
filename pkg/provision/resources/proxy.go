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
	configv1 "github.com/openshift/api/config/v1"
)

const (
	ProxyResourceName = "cluster"
)

var Proxy = &proxyHandler{}

type proxyHandler struct {
}

func (h *proxyHandler) Get(cheCtx *chetypes.CheContext, proxy *configv1.Proxy) (bool, error) {
	return syncher.GetObject(cheCtx, ProxyResourceName, "", proxy)
}
