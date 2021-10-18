//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package checlusterbackup

import (
	"context"
	"fmt"
	"os"
	"time"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	BackupCheEclipseOrg = "backup.che.eclipse.org"

	backupDestDir = "/tmp/che-backup-data"
)

// ReconcileCheClusterBackup reconciles a CheClusterBackup object
type ReconcileCheClusterBackup struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	// the namespace to which to limit the reconciliation. If empty, all namespaces are considered
	namespace string
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(mgr manager.Manager, namespace string) *ReconcileCheClusterBackup {
	return &ReconcileCheClusterBackup{client: mgr.GetClient(), scheme: mgr.GetScheme(), namespace: namespace}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReconcileCheClusterBackup) SetupWithManager(mgr ctrl.Manager) error {
	// Filter events to allow only create event on backup CR to trigger a new backup process
	backupCRPredicate := predicate.Funcs{
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return false
		},
		CreateFunc: func(evt event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(evt event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(evt event.GenericEvent) bool {
			return false
		},
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		Named("checlusterbackup-controller").
		Watches(&source.Kind{Type: &chev1.CheClusterBackup{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(backupCRPredicate))

	if r.namespace != "" {
		bldr = bldr.WithEventFilter(util.InNamespaceEventFilter(r.namespace))
	}

	return bldr.
		For(&chev1.CheClusterBackup{}).
		Complete(r)
}

// Reconcile reads that state of the cluster for a CheClusterBackup object and makes changes based on the state read
// and what is in the CheClusterBackup.Spec
// Note: The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCheClusterBackup) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	// Fetch the CheClusterBackup instance
	backupCR := &chev1.CheClusterBackup{}
	err := r.client.Get(context.TODO(), request.NamespacedName, backupCR)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	done, err := r.doReconcile(backupCR)
	if err != nil {
		// Log the error, so user can see it in logs
		logrus.Error(err)
		if !done {
			// Reconcile because the job is not done yet.
			// Probably the problem is related to a network error, etc.
			return ctrl.Result{RequeueAfter: 1 * time.Second}, err
		}

		// Update backup CR status with the error
		backupCR.Status.Message = "Error: " + err.Error()
		backupCR.Status.State = chev1.STATE_FAILED
		backupCR.Status.SnapshotId = ""
		if err := r.UpdateCRStatus(backupCR); err != nil {
			// Failed to update status, retry
			return ctrl.Result{}, err
		}

		// Do not reconcile despite the fact that an error happened.
		// The error cannot be handled automatically by the operator, so the user has to deal with it in manual mode.
		// For example, config in the backup CR is invalid, so do not requeue as user has to correct it.
		// After a modification in the backup CR, a new reconcile loop will be trigerred.
		return ctrl.Result{}, nil
	}
	if !done {
		// There was no error, but it is required to proceed after some delay,
		// e.g wait until some resources are flushed and/or ready.
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// Job is done
	return ctrl.Result{}, nil
}

func (r *ReconcileCheClusterBackup) doReconcile(backupCR *chev1.CheClusterBackup) (bool, error) {
	// Prevent any further action if backup process finished (succeeded or failed).
	// To restart restore process one need to recreate restore CR.
	if backupCR.Status.State != chev1.STATE_IN_PROGRESS && backupCR.Status.State != "" {
		return true, nil
	}

	// Validate backup CR
	if backupCR.Spec.BackupServerConfigRef == "" && !backupCR.Spec.UseInternalBackupServer {
		return true, fmt.Errorf("BackupServerConfigRef is not set, nor UseInternalBackupServer requested")
	}

	// Fetch backup server config, if any
	var backupServerConfigCR *chev1.CheBackupServerConfiguration
	if backupCR.Spec.BackupServerConfigRef != "" {
		backupServerConfigCR = &chev1.CheBackupServerConfiguration{}
		backupServerConfigNamespacedName := types.NamespacedName{Namespace: backupCR.GetNamespace(), Name: backupCR.Spec.BackupServerConfigRef}
		if err := r.client.Get(context.TODO(), backupServerConfigNamespacedName, backupServerConfigCR); err != nil {
			if errors.IsNotFound(err) {
				return true, fmt.Errorf("backup server configuration with name '%s' not found in '%s' namespace", backupCR.Spec.BackupServerConfigRef, backupCR.GetNamespace())
			}
			return false, err
		}
	}

	// Create backup context
	bctx, err := NewBackupContext(r, backupCR, backupServerConfigCR)
	if err != nil {
		// Failed to create backup context.
		// This is usually caused by invalid configuration of current backup server in the backup CR.
		// Do not requeue as user has to correct the configuration manually.
		return true, err
	}

	// Update status with progress on the first reconcile loop
	if bctx.backupCR.Status.State == "" {
		bctx.backupCR.Status.Message = "Backup is in progress. Start time: " + time.Now().String()
		bctx.backupCR.Status.State = chev1.STATE_IN_PROGRESS
		bctx.backupCR.Status.Phase = bctx.state.GetPhaseMessage()
		if err := r.UpdateCRStatus(backupCR); err != nil {
			return false, err
		}
	}

	// Check if internal backup server is needed
	if bctx.backupCR.Spec.UseInternalBackupServer {
		// Use internal REST backup server
		done, err := ConfigureInternalBackupServer(bctx)
		if err != nil || !done {
			return done, err
		}
	}

	// Update progress
	// If internal backup server is not needed, consider step is done
	bctx.state.internalBackupServerSetup = true
	bctx.UpdateBackupStatusPhase()

	// Make sure, that backup server configuration in the CR is valid and cache cluster resources
	done, err := bctx.backupServer.PrepareConfiguration(bctx.r.client, bctx.namespace)
	if err != nil || !done {
		return done, err
	}

	// Check for repository existance and init if needed
	repoExist, done, err := bctx.backupServer.IsRepositoryExist()
	if err != nil || !done {
		return done, err
	}
	if !repoExist {
		done, err := bctx.backupServer.InitRepository()
		if err != nil || !done {
			return done, err
		}
	}

	// Check if credentials provided in the configuration can be used to reach backup server content
	done, err = bctx.backupServer.CheckRepository()
	if err != nil || !done {
		return done, err
	}

	// Update progress
	bctx.state.backupRepositoryReady = true
	bctx.UpdateBackupStatusPhase()

	// Schedule cleanup
	defer os.RemoveAll(backupDestDir)
	// Collect all needed data to backup
	done, err = CollectBackupData(bctx, backupDestDir)
	if err != nil || !done {
		return done, err
	}

	// Update progress
	bctx.state.cheInstallationBackupDataCollected = true
	bctx.UpdateBackupStatusPhase()

	// Upload collected data to backup server
	snapshotStat, done, err := bctx.backupServer.SendSnapshot(backupDestDir)
	if err != nil || !done {
		return done, err
	}

	// Backup is successfully done
	// Update status
	bctx.state.backupSnapshotSent = true
	bctx.backupCR.Status.Phase = bctx.state.GetPhaseMessage()
	bctx.backupCR.Status.Message = "Backup successfully finished at " + time.Now().String()
	bctx.backupCR.Status.State = chev1.STATE_SUCCEEDED
	bctx.backupCR.Status.SnapshotId = snapshotStat.Id
	bctx.backupCR.Status.CheVersion = bctx.cheCR.Status.CheVersion
	if err := bctx.r.UpdateCRStatus(bctx.backupCR); err != nil {
		logrus.Errorf("Failed to update status after successful backup: %v", err)
		return true, err
	}

	logrus.Info(bctx.backupCR.Status.Message)
	return true, nil
}

func (r *ReconcileCheClusterBackup) UpdateCR(cr *chev1.CheClusterBackup) error {
	err := r.client.Update(context.TODO(), cr)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", cr.Name, err.Error())
		return err
	}
	return nil
}

func (r *ReconcileCheClusterBackup) UpdateCRStatus(cr *chev1.CheClusterBackup) error {
	err := r.client.Status().Update(context.TODO(), cr)
	if err != nil {
		logrus.Errorf("Failed to update %s CR status: %s", cr.Name, err.Error())
		return err
	}
	return nil
}
