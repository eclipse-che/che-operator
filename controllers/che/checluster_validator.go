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
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// CheClusterValidator checks CheCluster CR configuration.
// It detect:
// - configurations which miss required field(s) to deploy Che
// - self-contradictory configurations
// - configurations with which it is impossible to deploy Che
type CheClusterValidator struct {
}

func NewCheClusterValidator() *CheClusterValidator {
	return &CheClusterValidator{}
}

func (v *CheClusterValidator) Register(rm *deploy.ReconcileManager) {
	rm.RegisterReconciler(v)
}

func (v *CheClusterValidator) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if !util.IsOpenShift {
		if ctx.CheCluster.Spec.K8s.IngressDomain == "" {
			return reconcile.Result{}, false, fmt.Errorf("Required field \"spec.K8s.IngressDomain\" is not set")
		}
	}

	workspaceNamespaceDefault := util.GetWorkspaceNamespaceDefault(ctx.CheCluster)
	if strings.Index(workspaceNamespaceDefault, "<username>") == -1 && strings.Index(workspaceNamespaceDefault, "<userid>") == -1 {
		return reconcile.Result{}, false, fmt.Errorf(`Namespace strategies other than 'per user' is not supported anymore. Using the <username> or <userid> placeholder is required in the 'spec.server.workspaceNamespaceDefault' field. The current value is: %s`, workspaceNamespaceDefault)
	}

	if !util.IsCheMultiUser(ctx.CheCluster) {
		return reconcile.Result{}, false, fmt.Errorf("Single user authentication mode is not supported anymore. To backup your data you can commit workspace configuration to an SCM server and use factories to restore it in multi user mode. To switch to multi user authentication mode set 'spec.server.customCheProperties.CHE_MULTIUSER' to 'true' in %s CheCluster custom resource. Switching to multi user authentication mode without backing up your data will cause data loss.", ctx.CheCluster.Name)
	}

	return reconcile.Result{}, true, nil
}

func (v *CheClusterValidator) Finalize(ctx *deploy.DeployContext) (bool, error) {
	return true, nil
}
