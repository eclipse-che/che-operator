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

package migration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	logger = ctrl.Log.WithName("checluster-defaults-cleaner")
)

const (
	cheClusterDefaultsCleanupAnnotation = "che.eclipse.org/checluster-defaults-cleanup"
)

// CheClusterDefaultsCleaner is a migration tool that cleans up the CheCluster CR by removing values
// that have been set by the operator in the past as defaults.
// The purpose of this are the following:
//   - productization needs, downstream version of the operator can have different defaults
//   - possibility to change defaults, it allows to have new values after upgrading the operator, because
//     previous ones are not relevant anymore and can't be changed once the CR is created

type CheClusterDefaultsCleaner struct {
	reconciler.Reconcilable
	actionTasks []ActionTask
}

type ActionTask struct {
	field      string
	actionFunc func(*chetypes.DeployContext) (bool, error)
}

func NewCheClusterDefaultsCleaner() *CheClusterDefaultsCleaner {
	return &CheClusterDefaultsCleaner{
		actionTasks: []ActionTask{
			{
				field:      "spec.devEnvironments.defaultEditor",
				actionFunc: cleanUpDevEnvironmentsDefaultEditor,
			},
			{
				field:      "spec.devEnvironments.defaultComponents",
				actionFunc: cleanUpDevEnvironmentsDefaultComponents,
			},
			{
				field:      "spec.devEnvironments.disableContainerBuildCapabilities",
				actionFunc: cleanUpDevEnvironmentsDisableContainerBuildCapabilities,
			},
			{
				field:      "spec.components.dashboard.headerMessage",
				actionFunc: cleanUpDashboardHeaderMessage,
			},
			{
				field:      "spec.components.pluginRegistry.openVSXURL",
				actionFunc: cleanUpPluginRegistryOpenVSXURL,
			},
			{
				field:      "containers.resources",
				actionFunc: cleanUpContainersResources,
			},
			{
				field:      "spec.devEnvironments.containerRunConfiguration.containerSecurityContext.capabilities.add",
				actionFunc: updateDevEnvironmentsContainerRunConfiguration,
			},
		},
	}
}

func (dc *CheClusterDefaultsCleaner) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	for _, actionTask := range dc.actionTasks {
		if dc.isFieldProcessed(ctx, actionTask.field) {
			continue
		}

		if !ctx.CheCluster.IsCheBeingInstalled() {
			if done, err := actionTask.actionFunc(ctx); err != nil {
				return reconcile.Result{}, false, err
			} else if done {
				logger.Info("CheCluster CR updated", "field", actionTask.field)
			}
			// done == false
			// means nothing to update
		}

		dc.setFieldProcessed(ctx, actionTask.field)
	}

	if err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster); err != nil {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (dc *CheClusterDefaultsCleaner) Finalize(_ *chetypes.DeployContext) bool {
	return true
}

func (dc *CheClusterDefaultsCleaner) isFieldProcessed(ctx *chetypes.DeployContext, field string) bool {
	fields := dc.getProcessedFields(ctx)
	return fields[field] == "true"
}

func (dc *CheClusterDefaultsCleaner) setFieldProcessed(ctx *chetypes.DeployContext, field string) {
	fields := dc.getProcessedFields(ctx)
	fields[field] = "true"

	data, err := json.Marshal(fields)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to marshal %s annotation", cheClusterDefaultsCleanupAnnotation))
	}

	annotations := utils.GetMapOrDefault(ctx.CheCluster.GetAnnotations(), map[string]string{})
	annotations[cheClusterDefaultsCleanupAnnotation] = string(data)
	ctx.CheCluster.SetAnnotations(annotations)
}

func (dc *CheClusterDefaultsCleaner) getProcessedFields(ctx *chetypes.DeployContext) map[string]string {
	annotations := utils.GetMapOrDefault(ctx.CheCluster.GetAnnotations(), map[string]string{})

	data := annotations[cheClusterDefaultsCleanupAnnotation]
	if data == "" {
		return map[string]string{}
	}

	fields := map[string]string{}
	if err := json.Unmarshal([]byte(data), &fields); err != nil {
		logger.Error(err, fmt.Sprintf("Failed to unmarshal %s annotation", cheClusterDefaultsCleanupAnnotation))
	}

	return fields
}
