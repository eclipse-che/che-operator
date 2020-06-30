//
// Copyright (c) 2020-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package deploy

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProvisioningStatus struct {
	Continue bool
	Requeue  bool
	Err      error
}

type ClusterAPI struct {
	Client client.Client
	Scheme *runtime.Scheme
}

type Proxy struct {
	HttpProxy    string
	HttpUser     string
	HttpPassword string
	HttpHost     string
	HttpPort     string

	HttpsProxy    string
	HttpsUser     string
	HttpsPassword string
	HttpsHost     string
	HttpsPort     string

	NoProxy          string
	TrustedCAMapName string
}
