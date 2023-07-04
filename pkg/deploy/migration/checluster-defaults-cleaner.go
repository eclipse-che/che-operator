//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	logger = ctrl.Log.WithName("checluster-defaults-cleaner")
)

const (
	cheClusterDefaultsCleanup = "che.eclipse.org/checluster-defaults-cleanup"
)

// CheClusterDefaultsCleaner is a migration tool that cleans up the CheCluster CR by removing values
// that have been set by the operator in the past as defaults.
// All those defaults are moved to environment variables in the operator deployment now.
// The purpose of this are the following:
// - productization needs, downstream version of the operator can have different defaults
// - possibility to change defaults, it allows to have new values after upgrading the operator, because
//   previous ones are not relevant anymore and can't be changed once the CR is created
type CheClusterDefaultsCleaner struct {
	deploy.Reconcilable
	cleanUpTasks []CleanUpTask
}

type CleanUpTask struct {
	cleanUpFunc      func(*chetypes.DeployContext) (bool, error)
	fieldsIdentifier string
}

func NewCheClusterDefaultsCleaner() *CheClusterDefaultsCleaner {
	return &CheClusterDefaultsCleaner{
		cleanUpTasks: []CleanUpTask{
			{
				cleanUpFunc:      cleanUpDevEnvironmentsDefaultEditor,
				fieldsIdentifier: "spec.devEnvironments.defaultEditor",
			},
			{
				cleanUpFunc:      cleanUpDevEnvironmentsDefaultComponents,
				fieldsIdentifier: "spec.devEnvironments.defaultComponents",
			},
			{
				cleanUpFunc:      cleanUpDevEnvironmentsDisableContainerBuildCapabilities,
				fieldsIdentifier: "spec.devEnvironments.disableContainerBuildCapabilities",
			},
			{
				cleanUpFunc:      cleanUpDashboardHeaderMessage,
				fieldsIdentifier: "spec.components.dashboard.headerMessage",
			},
			{
				cleanUpFunc:      cleanUpPluginRegistryOpenVSXURL,
				fieldsIdentifier: "spec.components.pluginRegistry.openVSXURL",
			},
			{
				cleanUpFunc:      cleanUpContainersResources,
				fieldsIdentifier: "containers.resources",
			},
		},
	}
}

func (dc *CheClusterDefaultsCleaner) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	for _, cleanUpTask := range dc.cleanUpTasks {
		if dc.isCheClusterDefaultsCleanupAnnotationSet(ctx, cleanUpTask.fieldsIdentifier) {
			continue
		}

		if !ctx.CheCluster.IsCheBeingInstalled() {
			if done, err := cleanUpTask.cleanUpFunc(ctx); err != nil {
				return reconcile.Result{}, false, err
			} else if done {
				logger.Info("CheCluster CR cleaned up", "field", cleanUpTask.fieldsIdentifier)
			}
		}

		// set annotation to mark that the field has been processed
		dc.setCheClusterDefaultsCleanupAnnotation(ctx, cleanUpTask.fieldsIdentifier)
		if err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (dc *CheClusterDefaultsCleaner) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (dc *CheClusterDefaultsCleaner) isCheClusterDefaultsCleanupAnnotationSet(ctx *chetypes.DeployContext, cheClusterField string) bool {
	cheClusterFields := dc.readCheClusterDefaultsCleanupAnnotation(ctx)
	return cheClusterFields[cheClusterField] == "true"
}

func (dc *CheClusterDefaultsCleaner) setCheClusterDefaultsCleanupAnnotation(ctx *chetypes.DeployContext, cheClusterField string) {
	cheClusterFields := dc.readCheClusterDefaultsCleanupAnnotation(ctx)
	cheClusterFields[cheClusterField] = "true"

	data, err := json.Marshal(cheClusterFields)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to marshal %s annotation", cheClusterDefaultsCleanup))
	}

	annotations := ctx.CheCluster.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{
			cheClusterDefaultsCleanup: string(data),
		}
	} else {
		annotations[cheClusterDefaultsCleanup] = string(data)
	}

	ctx.CheCluster.SetAnnotations(annotations)
}

func (dc *CheClusterDefaultsCleaner) readCheClusterDefaultsCleanupAnnotation(ctx *chetypes.DeployContext) map[string]string {
	annotations := ctx.CheCluster.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	cheClusterFields := map[string]string{}
	if annotations[cheClusterDefaultsCleanup] != "" {
		if err := json.Unmarshal([]byte(annotations[cheClusterDefaultsCleanup]), &cheClusterFields); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to unmarshal %s annotation", cheClusterDefaultsCleanup))
		}
	}

	return cheClusterFields
}
