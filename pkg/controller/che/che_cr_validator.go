//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package che

import (
	"fmt"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

// ValidateCheCR checks Che CR configuration.
// It should detect:
// - configurations which miss required field(s) to deploy Che
// - self-contradictory configurations
// - configurations with which it is impossible to deploy Che
func ValidateCheCR(checluster *orgv1.CheCluster, isOpenshift bool) (bool, string) {
	var isValid bool
	var errorMessage string

	if !isOpenshift {
		if checluster.Spec.K8s.IngressDomain == "" {
			return false, fmt.Sprintf("Required parameter \"Spec.K8s.IngressDomain\" is not set.")
		}
	}

	isValid, errorMessage = checkTLSConfiguration(checluster, isOpenshift)
	if !isValid {
		return isValid, errorMessage
	}

	return true, ""
}

func checkTLSConfiguration(checluster *orgv1.CheCluster, isOpenshift bool) (bool, string) {
	if !checluster.Spec.Server.TlsSupport {
		return true, ""
	}

	if !isOpenshift {
		// Check TLS secret name is set
		if checluster.Spec.K8s.TlsSecretName == "" {
			return false, fmt.Sprintf("TLS is enabled, but required parameter \"Spec.K8s.TlsSecretName\" is not set.")
		}
	}

	return true, ""
}
