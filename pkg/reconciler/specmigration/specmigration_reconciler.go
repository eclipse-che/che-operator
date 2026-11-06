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

package specmigration

import (
	"encoding/json"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var logger = ctrl.Log.WithName("specmigration")

const cheClusterSpecMigrationAnnotation = "che.eclipse.org/checluster-defaults-cleanup"

// SpecMigrationReconciler is a migration tool that ensures CheCluster CR fields carry
// required defaults and removes values that were set by
// the operator in the past as defaults but are not relevant anymore.
type SpecMigrationReconciler struct {
	reconciler.Reconcilable
}

type migrationTask struct {
	field         string
	migrationFunc func(*chetypes.CheContext) error
}

var migrationTasks = []migrationTask{
	{
		// No cleanup needed anymore: it was performed long ago.
		// The entry is kept only for history.
		field: "spec.devEnvironments.defaultEditor",
	},
	{
		// No cleanup needed anymore: it was performed long ago.
		// The entry is kept only for history.
		field: "spec.devEnvironments.defaultComponents",
	},
	{
		// No cleanup needed anymore: it was performed long ago.
		// The entry is kept only for history.
		field: "spec.devEnvironments.disableContainerBuildCapabilities",
	},
	{
		// No cleanup needed anymore: it was performed long ago.
		// The entry is kept only for history.
		field: "spec.components.dashboard.headerMessage",
	},
	{
		// No cleanup needed anymore: it was performed long ago.
		// The entry is kept only for history.
		field: "spec.components.pluginRegistry.openVSXURL",
	},
	{
		// No cleanup needed anymore: it was performed long ago.
		// The entry is kept only for history.
		field: "containers.resources",
	},
	{
		field:         "spec.devEnvironments.containerRunConfiguration.containerSecurityContext.capabilities.add",
		migrationFunc: addChownCapability,
	},
}

func NewReconciler() *SpecMigrationReconciler {
	return &SpecMigrationReconciler{}
}

func (r *SpecMigrationReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	err := r.runMigrationTasks(cheCtx)
	if err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to migrate spec fields: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (r *SpecMigrationReconciler) Finalize(_ *chetypes.CheContext) error {
	return nil
}

func (r *SpecMigrationReconciler) runMigrationTasks(cheCtx *chetypes.CheContext) error {
	// All migrationFunc implementations are idempotent, so it is safe to batch
	// all tasks into a single sync. A crash before the sync causes a retry on
	// the next reconcile; no task needs per-task persistence.
	updated := false
	fields := r.getCompletedFields(cheCtx)

	for _, task := range migrationTasks {
		if fields[task.field] == "true" {
			continue
		}

		if task.migrationFunc == nil {
			continue
		}

		if !cheCtx.CheCluster.IsCheBeingInstalled() {
			// In case of upgrades only
			if err := task.migrationFunc(cheCtx); err != nil {
				return err
			}
		}

		updated = true
		fields[task.field] = "true"
	}

	if updated {
		if err := r.setCompletedFields(cheCtx, fields); err != nil {
			return err
		}
		return resources.CheCluster.Sync(cheCtx)
	}

	return nil
}

func (r *SpecMigrationReconciler) setCompletedFields(cheCtx *chetypes.CheContext, fields map[string]string) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("could not marshal fields: %w", err)
	}

	annotations := utils.NonNilMap(cheCtx.CheCluster.GetAnnotations())
	annotations[cheClusterSpecMigrationAnnotation] = string(data)
	cheCtx.CheCluster.SetAnnotations(annotations)

	return nil
}

func (r *SpecMigrationReconciler) getCompletedFields(cheCtx *chetypes.CheContext) map[string]string {
	annotations := utils.NonNilMap(cheCtx.CheCluster.GetAnnotations())

	data := annotations[cheClusterSpecMigrationAnnotation]
	if data == "" {
		return map[string]string{}
	}

	fields := map[string]string{}
	if err := json.Unmarshal([]byte(data), &fields); err != nil {
		// Don't break the reconciler on a corrupted annotation: treat it as empty
		// so the update tasks are simply re-evaluated and the annotation is
		// rewritten with valid data on the next sync.
		logger.Error(err, "Failed to unmarshal annotation", "annotation", cheClusterSpecMigrationAnnotation)
	}

	return utils.NonNilMap(fields)
}
