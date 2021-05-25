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
package checlusterrestore

import (
	"context"
	"fmt"
	"os"
	"time"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new CheClusterRestore Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCheClusterRestore{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("checlusterrestore-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource CheClusterRestore
	err = c.Watch(&source.Kind{Type: &orgv1.CheClusterRestore{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileCheClusterRestore implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCheClusterRestore{}

// ReconcileCheClusterRestore reconciles a CheClusterRestore object
type ReconcileCheClusterRestore struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

const (
	backupDataDestDir = "/tmp/che-restore-data"
)

// Reconcile reads that state of the cluster for a CheClusterRestore object and makes changes based on the state read
// and what is in the CheClusterRestore.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCheClusterRestore) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the CheClusterRestore instance
	restoreCR := &orgv1.CheClusterRestore{}
	err := r.client.Get(context.TODO(), request.NamespacedName, restoreCR)
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

	done, err := r.doReconcile(restoreCR)
	if err != nil {
		// Log the error, so user can see it in logs
		logrus.Error(err)
		if !done {
			// Reconcile because the job is not done yet.
			// Probably the problem is related to a network error, etc.
			return reconcile.Result{}, err
		}

		// Update restore CR status with the error
		restoreCR.Status.Message = "Error: " + err.Error()
		if err := r.UpdateCRStatus(restoreCR); err != nil {
			// Failed to update status, retry
			return reconcile.Result{}, err
		}

		// Do not reconcile despite the fact that an error happened.
		// The error cannot be handled automatically by the operator, so the user has to deal with it in manual mode.
		// For example, config in the restore CR is invalid, so do not requeue as user has to correct it.
		// After a modification in the restore CR, a new reconcile loop will be trigerred.
		return reconcile.Result{}, nil
	}
	if !done {
		// There was no error, but it is required to proceed after some delay,
		// e.g wait until some resources are flushed and/or ready.
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Job is done
	return reconcile.Result{}, nil
}

func (r *ReconcileCheClusterRestore) doReconcile(restoreCR *orgv1.CheClusterRestore) (bool, error) {
	if restoreCR.Spec.CopyBackupServerConfiguration {
		done, err := r.copyBackupServersConfiguration(restoreCR)
		if err != nil || !done {
			return done, err
		}

		restoreCR.Spec.CopyBackupServerConfiguration = false
		if err := r.UpdateCR(restoreCR); err != nil {
			return false, err
		}
	}

	rctx, err := NewRestoreContext(r, restoreCR)
	if err != nil {
		// Failed to create context.
		// This is usually caused by invalid configuration of current backup server in the restore CR.
		// Do not requeue as user has to correct the configuration manually.
		return true, err
	}

	// Make sure, that backup server configuration in the CR is valid and cache cluster resources
	done, err := rctx.backupServer.PrepareConfiguration(rctx.r.client, rctx.namespace)
	if err != nil || !done {
		return done, err
	}

	if rctx.restoreCR.Spec.TriggerNow {
		rctx.UpdateRestoreStage()

		if !rctx.state.backupDownloaded {
			// Check if repository accesible and credentials provided in the configuration can be used to reach backup server content
			done, err = rctx.backupServer.CheckRepository()
			if err != nil || !done {
				return done, err
			}

			if err := os.RemoveAll(backupDataDestDir); err != nil {
				return false, err
			}
			// Download data from backup server
			if rctx.restoreCR.Spec.SnapshotId != "" {
				done, err = rctx.backupServer.DownloadSnapshot(rctx.restoreCR.Spec.SnapshotId, backupDataDestDir)
			} else {
				done, err = rctx.backupServer.DownloadLastSnapshot(backupDataDestDir)
			}
			if err != nil || !done {
				return done, err
			}
			logrus.Info("Restore: Retrieved data from backup server")
			rctx.state.backupDownloaded = true
			rctx.UpdateRestoreStage()
		}

		if !rctx.state.cheRestored {
			// Restore all data from the backup
			done, err = RestoreChe(rctx, backupDataDestDir)
			if err != nil || !done {
				return done, err
			}

			rctx.state.cheRestored = true
			rctx.UpdateRestoreStage()

			// Clean up backup data after successful restore
			if err := os.RemoveAll(backupDataDestDir); err != nil {
				return false, err
			}
		}

		// Restore is successfull
		rctx.restoreCR.Spec.TriggerNow = false
		if err := rctx.r.UpdateCR(rctx.restoreCR); err != nil {
			return false, err
		}

		rctx.restoreCR.Status.Message = "Restore successfully finished"
		if err := rctx.r.UpdateCRStatus(rctx.restoreCR); err != nil {
			return false, err
		}

		if rctx.restoreCR.Spec.DeleteConfigurationAfterRestore {
			if err := rctx.r.client.Delete(context.TODO(), rctx.restoreCR); err != nil {
				return true, err
			}
		}

		logrus.Info("Restore successfully finished")
	}
	// Reset state to restart the restore flow on the next reconcile
	rctx.state.Reset()

	return true, nil
}

func (r *ReconcileCheClusterRestore) copyBackupServersConfiguration(restoreCR *orgv1.CheClusterRestore) (bool, error) {
	backupCRs := &orgv1.CheClusterBackupList{}
	if err := r.client.List(context.TODO(), backupCRs); err != nil {
		return false, err
	}

	if len(backupCRs.Items) != 1 {
		if len(backupCRs.Items) == 0 {
			return true, fmt.Errorf("cannot copy backup servers configuration: backup CR not found")
		}
		return true, fmt.Errorf("expected an instance of CheClusterBackup, but got %d instances", len(backupCRs.Items))
	}

	backupCR := &orgv1.CheClusterBackup{}
	namespacedName := types.NamespacedName{Namespace: restoreCR.GetNamespace(), Name: backupCRs.Items[0].GetName()}
	if err := r.client.Get(context.TODO(), namespacedName, backupCR); err != nil {
		return false, err
	}

	restoreCR.Spec.Servers = backupCR.Spec.Servers
	if backupCR.Spec.ServerType != "" {
		restoreCR.Spec.ServerType = backupCR.Spec.ServerType
	}
	return true, nil
}

func (r *ReconcileCheClusterRestore) UpdateCR(cr *orgv1.CheClusterRestore) error {
	err := r.client.Update(context.TODO(), cr)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", cr.Name, err.Error())
		return err
	}
	return nil
}

func (r *ReconcileCheClusterRestore) UpdateCRStatus(cr *orgv1.CheClusterRestore) error {
	err := r.client.Status().Update(context.TODO(), cr)
	if err != nil {
		logrus.Errorf("Failed to update %s CR status: %s", cr.Name, err.Error())
		return err
	}
	return nil
}
