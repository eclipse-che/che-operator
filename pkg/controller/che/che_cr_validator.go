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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
)

// ValidateCheCR checks Che CR configuration.
// It should detect:
// - configurations which miss required field(s) to deploy Che
// - self-contradictory configurations
// - configurations with which it is impossible to deploy Che
func ValidateCheCR(checluster *orgv1.CheCluster, isOpenshift bool) error {
	if !isOpenshift {
		if checluster.Spec.K8s.IngressDomain == "" {
			return fmt.Errorf("Required field \"spec.K8s.IngressDomain\" is not set")
		}
	}

	return nil
}
