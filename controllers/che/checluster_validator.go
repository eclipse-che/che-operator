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

package che

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CheClusterValidator checks CheCluster CR configuration.
// It detects:
// - configurations which miss required field(s) to deploy Che
type CheClusterValidator struct {
	deploy.Reconcilable
}

func NewCheClusterValidator() *CheClusterValidator {
	return &CheClusterValidator{}
}

func (v *CheClusterValidator) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !infrastructure.IsOpenShift() {
		if ctx.CheCluster.Spec.Networking.Domain == "" {
			return reconcile.Result{}, false, fmt.Errorf("Required field \"spec.networking.domain\" is not set")
		}
	}

	return reconcile.Result{}, true, nil
}

func (v *CheClusterValidator) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}
