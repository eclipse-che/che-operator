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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var logger = ctrl.Log.WithName("checluster-defaults-cleaner")

const cheClusterDefaultsCleanupAnnotation = "che.eclipse.org/checluster-defaults-cleanup"

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
	field    string
	doUpdate func(*chetypes.CheContext) (bool, error)
}

func NewCheClusterDefaultsCleaner() *CheClusterDefaultsCleaner {
	return &CheClusterDefaultsCleaner{
		actionTasks: []ActionTask{
			{
				field:    "spec.devEnvironments.defaultEditor",
				doUpdate: cleanUpDevEnvironmentsDefaultEditor,
			},
			{
				field:    "spec.devEnvironments.defaultComponents",
				doUpdate: cleanUpDevEnvironmentsDefaultComponents,
			},
			{
				field:    "spec.devEnvironments.disableContainerBuildCapabilities",
				doUpdate: cleanUpDevEnvironmentsDisableContainerBuildCapabilities,
			},
			{
				field:    "spec.components.dashboard.headerMessage",
				doUpdate: cleanUpDashboardHeaderMessage,
			},
			{
				field:    "spec.components.pluginRegistry.openVSXURL",
				doUpdate: cleanUpPluginRegistryOpenVSXURL,
			},
			{
				field:    "containers.resources",
				doUpdate: cleanUpContainersResources,
			},
			{
				field:    "spec.devEnvironments.containerRunConfiguration.containerSecurityContext.capabilities.add",
				doUpdate: updateDevEnvironmentsContainerRunConfiguration,
			},
		},
	}
}

func (dc *CheClusterDefaultsCleaner) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	for _, actionTask := range dc.actionTasks {
		if dc.isFieldProcessed(cheCtx, actionTask.field) {
			continue
		}

		if !cheCtx.CheCluster.IsCheBeingInstalled() {
			done, err := actionTask.doUpdate(cheCtx)
			if done {
				logger.Info("CheCluster CR updated", "field", actionTask.field)
			} else if err != nil {
				return reconcile.Result{}, false, err
			}
		}

		dc.setFieldProcessed(cheCtx, actionTask.field)
		err := cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
		if err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (dc *CheClusterDefaultsCleaner) Finalize(_ *chetypes.CheContext) bool {
	return true
}

func (dc *CheClusterDefaultsCleaner) isFieldProcessed(cheCtx *chetypes.CheContext, field string) bool {
	fields := dc.getProcessedFields(cheCtx)
	return fields[field] == "true"
}

func (dc *CheClusterDefaultsCleaner) setFieldProcessed(cheCtx *chetypes.CheContext, field string) {
	fields := dc.getProcessedFields(cheCtx)
	fields[field] = "true"

	data, err := json.Marshal(fields)
	if err != nil {
		logger.Error(err, "Failed to marshal annotation", "annotation", cheClusterDefaultsCleanupAnnotation)
	}

	annotations := utils.GetMapOrDefault(cheCtx.CheCluster.GetAnnotations(), map[string]string{})
	annotations[cheClusterDefaultsCleanupAnnotation] = string(data)
	cheCtx.CheCluster.SetAnnotations(annotations)
}

func (dc *CheClusterDefaultsCleaner) getProcessedFields(cheCtx *chetypes.CheContext) map[string]string {
	annotations := utils.GetMapOrDefault(cheCtx.CheCluster.GetAnnotations(), map[string]string{})

	data := annotations[cheClusterDefaultsCleanupAnnotation]
	if data == "" {
		return map[string]string{}
	}

	fields := map[string]string{}
	if err := json.Unmarshal([]byte(data), &fields); err != nil {
		logger.Error(err, "Failed to unmarshal annotation", "annotation", cheClusterDefaultsCleanupAnnotation)
	}

	return fields
}
