//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package pluginregistry

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"golang.org/x/mod/semver"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	v7530             = "v7.53.0"
	openVSXDefaultUrl = "https://open-vsx.org"
)

type OpenVSXUrlReconciler struct {
	deploy.Reconcilable
}

func NewOpenVSXUrlReconciler() *OpenVSXUrlReconciler {
	return &OpenVSXUrlReconciler{}
}

// https://github.com/eclipse/che/issues/21637
// When installing Che, the default CheCluster should have pluginRegistry.openVSXURL set to https://open-vsx.org.
// When updating Che v7.51 or earlier, if `openVSXURL` is NOT set then we should set it to https://open-vsx.org.
// When updating Che v7.52 or later, if `openVSXURL` is NOT set then we should not modify it.

func (r *OpenVSXUrlReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	done, err := r.syncOpenVSXUrl(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXUrlReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (r *OpenVSXUrlReconciler) syncOpenVSXUrl(ctx *chetypes.DeployContext) (bool, error) {
	if ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL == "" {
		if ctx.CheCluster.Status.CheVersion == "" {
			// installing Eclipse Che, set a default openVSX URL
			return setOpenVSXDefaultUrl(ctx)
		} else if ctx.CheCluster.Status.CheVersion == "next" {
			// do nothing for `next` version, because it considers as greater than 7.52.0
			return true, nil
		}

		if semver.Compare(fmt.Sprintf("v%s", ctx.CheCluster.Status.CheVersion), v7530) == -1 {
			// updating Eclipse Che, version is less than 7.53.0
			return setOpenVSXDefaultUrl(ctx)
		}
	}

	return true, nil
}

func setOpenVSXDefaultUrl(ctx *chetypes.DeployContext) (bool, error) {
	ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL = openVSXDefaultUrl
	if err := deploy.UpdateCheCRSpec(ctx, "openVSXURL", openVSXDefaultUrl); err != nil {
		return false, err
	}

	return true, nil
}
