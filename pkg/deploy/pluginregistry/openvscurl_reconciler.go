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
	v7520             = "v7.52.0"
	openVSXDefaultUrl = "https://openvsx.org"
)

type OpenVSXUrlReconciler struct {
	deploy.Reconcilable
}

func NewOpenVSXUrlReconciler() *OpenVSXUrlReconciler {
	return &OpenVSXUrlReconciler{}
}

// https://github.com/eclipse/che/issues/21637
// When installing Che, the default CheCluster should have pluginRegistry.openVSXURL set to https://openvsx.org.
// When updating Che v7.51 or earlier, if `openVSXURL` is NOT set then we should set it to https://openvsx.org.
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
	if ctx.CheCluster.Status.OpenVSXURL != ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL {
		if done, err := setOpenVSXUrl(ctx, ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL); !done {
			return false, err
		}
	}

	if ctx.CheCluster.Status.OpenVSXURL == "" {
		if ctx.CheCluster.Status.CheVersion == "" {
			// installing Eclipse Che, set a default openVSX URL
			return setOpenVSXUrl(ctx, openVSXDefaultUrl)
		} else if ctx.CheCluster.Status.CheVersion == "next" {
			// do nothing for `next` version, because it considers as greater than 7.52.0
			return true, nil
		}

		if semver.Compare(fmt.Sprintf("v%s", ctx.CheCluster.Status.CheVersion), v7520) == -1 {
			// updating Eclipse Che, version is less than 7.52.0
			return setOpenVSXUrl(ctx, openVSXDefaultUrl)
		}
	}

	return true, nil
}

func setOpenVSXUrl(ctx *chetypes.DeployContext, openVSXURL string) (bool, error) {
	ctx.CheCluster.Status.OpenVSXURL = openVSXURL
	if err := deploy.UpdateCheCRStatus(ctx, "openVSXURL", openVSXURL); err != nil {
		return false, err
	}

	return true, nil
}
