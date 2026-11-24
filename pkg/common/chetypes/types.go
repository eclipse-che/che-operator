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

package chetypes

import (
	"context"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/cheproxy"
	"github.com/eclipse-che/che-operator/pkg/common/clusterapi"
)

type CheContext struct {
	CheCluster     *chev2.CheCluster
	ClusterAPI     *clusterapi.ClusterAPI
	Proxy          *cheproxy.Proxy
	Authentication *Authentication
	CheHost        string
	DWONamespace   string
	Context        context.Context
}

type Authentication struct {
	GroupsClaim    string
	GroupsPrefix   string
	UsernameClaim  string
	UsernamePrefix string

	IssuerURL string
	IssuerCA  string

	ClientId     string
	ClientSecret []byte
}
