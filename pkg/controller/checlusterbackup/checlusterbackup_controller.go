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
	"os"
	"time"

	chev1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new CheClusterBackup Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCheClusterBackup{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("checlusterbackup-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CheClusterBackup
	err = c.Watch(&source.Kind{Type: &chev1.CheClusterBackup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

const (
	BackupCheEclipseOrg = "backup.che.eclipse.org"

	backupDestDir = "/tmp/che-backup-data"
)

// blank assignment to verify that ReconcileCheClusterBackup implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCheClusterBackup{}

// ReconcileCheClusterBackup reconciles a CheClusterBackup object
type ReconcileCheClusterBackup struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a CheClusterBackup object and makes changes based on the state read
// and what is in the CheClusterBackup.Spec
// Note: The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCheClusterBackup) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CheClusterBackup instance
	backupCR := &chev1.CheClusterBackup{}
	err := r.client.Get(context.TODO(), request.NamespacedName, backupCR)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	done, err := r.doReconcile(backupCR)
	if err != nil {
		// Log the error, so user can see it in logs
		logrus.Error(err)
		if !done {
			// Reconcile because the job is not done yet.
			// Probably the problem is related to a network error, etc.
			return reconcile.Result{}, err
		}

		// Update backup CR status with the error
		backupCR.Status.Message = "Error: " + err.Error()
		backupCR.Status.State = chev1.STATE_FAILED
		backupCR.Status.SnapshotId = ""
		if err := r.UpdateCRStatus(backupCR); err != nil {
			// Failed to update status, retry
			return reconcile.Result{}, err
		}

		// Do not reconcile despite the fact that an error happened.
		// The error cannot be handled automatically by the operator, so the user has to deal with it in manual mode.
		// For example, config in the backup CR is invalid, so do not requeue as user has to correct it.
		// After a modification in the backup CR, a new reconcile loop will be trigerred.
		return reconcile.Result{}, nil
	}
	if !done {
		// There was no error, but it is required to proceed after some delay,
		// e.g wait until some resources are flushed and/or ready.
		return reconcile.Result{RequeueAfter: 1 * time.Second}, nil
	}

	// Job is done
	return reconcile.Result{}, nil
}

func (r *ReconcileCheClusterBackup) doReconcile(backupCR *chev1.CheClusterBackup) (bool, error) {
	// Create backup context
	bctx, err := NewBackupContext(r, backupCR)
	if err != nil {
		// Failed to create backup context.
		// This is usually caused by invalid configuration of current backup server in the backup CR.
		// Do not requeue as user has to correct the configuration manually.
		return true, err
	}

	// Check if internal backup server is needed
	if bctx.backupCR.Spec.UseInternalBackupServer {
		// Use internal REST backup server
		done, err := ConfigureInternalBackupServer(bctx)
		if err != nil || !done {
			return done, err
		}
	}

	// Make sure, that backup server configuration in the CR is valid and cache cluster resources
	done, err := bctx.backupServer.PrepareConfiguration(bctx.r.client, bctx.namespace)
	if err != nil || !done {
		return done, err
	}

	// Do backup if requested
	if bctx.backupCR.Spec.TriggerNow {
		// Update status
		if bctx.backupCR.Status.State != chev1.STATE_IN_PROGRESS {
			bctx.backupCR.Status.Message = "Backup is in progress. Start time: " + time.Now().String()
			bctx.backupCR.Status.State = chev1.STATE_IN_PROGRESS
			bctx.backupCR.Status.SnapshotId = ""
			if err := bctx.r.UpdateCRStatus(bctx.backupCR); err != nil {
				return false, err
			}
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

		// Schedule cleanup
		defer os.RemoveAll(backupDestDir)
		// Collect all needed data to backup
		done, err = CollectBackupData(bctx, backupDestDir)
		if err != nil || !done {
			return done, err
		}

		// Upload collected data to backup server
		snapshotStat, done, err := bctx.backupServer.SendSnapshot(backupDestDir)
		if err != nil || !done {
			return done, err
		}

		// Backup is successfull
		bctx.backupCR.Spec.TriggerNow = false
		if err := bctx.r.UpdateCR(bctx.backupCR); err != nil {
			// Wait a bit and retry.
			// This is needed because actual backup is done successfully, but the CR still has backup flag set.
			// This update is important, because without it next reconcile loop will start a new backup.
			time.Sleep(5 * time.Second)
			if err := bctx.r.UpdateCR(bctx.backupCR); err != nil {
				return false, err
			}
		}

		// Update status
		bctx.backupCR.Status.Message = "Backup successfully finished at " + time.Now().String()
		bctx.backupCR.Status.State = chev1.STATE_SUCCEEDED
		bctx.backupCR.Status.SnapshotId = snapshotStat.Id
		if err := bctx.r.UpdateCRStatus(bctx.backupCR); err != nil {
			logrus.Errorf("Failed to update status after successful backup")
			// Do not reconcile as backup is done, only status is not updated
			return true, err
		}

		logrus.Info(bctx.backupCR.Status.Message)
	}

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
