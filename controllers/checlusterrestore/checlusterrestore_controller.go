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
package checlusterrestore

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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	backupDataDestDir = "/tmp/che-restore-data"
)

// ReconcileCheClusterRestore reconciles a CheClusterRestore object
type ReconcileCheClusterRestore struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	cachingClient    client.Client
	nonCachingClient client.Client
	scheme           *runtime.Scheme
	// the namespace to which to limit the reconciliation. If empty, all namespaces are considered
	namespace string
}

// NewReconciler returns a new reconcile.Reconciler
func NewReconciler(cachingClient client.Client, noncachingClient client.Client, scheme *runtime.Scheme, namespace string) *ReconcileCheClusterRestore {
	return &ReconcileCheClusterRestore{cachingClient: cachingClient, nonCachingClient: noncachingClient, scheme: scheme, namespace: namespace}
}

func (r *ReconcileCheClusterRestore) SetupWithManager(mgr ctrl.Manager) error {
	// Filter events to allow only create event on restore CR trigger a new reconcile loop
	restoreCRPredicate := predicate.Funcs{
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
		Named("checlusterrestore-controller").
		Watches(&source.Kind{Type: &chev1.CheClusterRestore{}}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(restoreCRPredicate))

	if r.namespace != "" {
		bldr = bldr.WithEventFilter(util.InNamespaceEventFilter(r.namespace))
	}

	return bldr.
		For(&chev1.CheClusterRestore{}).
		Complete(r)
}

// Reconcile reads that state of the cluster for a CheClusterRestore object and makes changes based on the state read
// and what is in the CheClusterRestore.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCheClusterRestore) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	// Fetch the CheClusterRestore instance
	restoreCR := &chev1.CheClusterRestore{}
	err := r.cachingClient.Get(context.TODO(), request.NamespacedName, restoreCR)
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

	done, err := r.doReconcile(restoreCR)
	if err != nil {
		// Log the error, so user can see it in logs
		logrus.Error(err)
		if !done {
			// Reconcile because the job is not done yet.
			// Probably the problem is related to a network error, etc.
			return ctrl.Result{RequeueAfter: 1 * time.Second}, err
		}

		// Update restore CR status with the error
		restoreCR.Status.Message = "Error: " + err.Error()
		restoreCR.Status.State = chev1.STATE_FAILED
		if err := r.UpdateCRStatus(restoreCR); err != nil {
			// Failed to update status, retry
			return ctrl.Result{}, err
		}

		// Do not reconcile despite the fact that an error happened.
		// The error cannot be handled automatically by the operator, so the user has to deal with it in manual mode.
		// For example, config in the restore CR is invalid, so do not requeue as user has to correct it.
		// After a modification in the restore CR, a new reconcile loop will be trigerred.
		return ctrl.Result{}, nil
	}
	if !done {
		// There was no error, but it is required to proceed after some delay,
		// e.g wait until some resources are flushed and/or ready.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Job is done
	return ctrl.Result{}, nil
}

func (r *ReconcileCheClusterRestore) doReconcile(restoreCR *chev1.CheClusterRestore) (done bool, err error) {
	// Prevent any further action if restore process finished (succeeded or failed).
	// To restart restore process one need to recreate restore CR.
	if restoreCR.Status.State != chev1.STATE_IN_PROGRESS && restoreCR.Status.State != "" {
		return true, nil
	}

	// Fetch backup server config
	var backupServerConfigCR *chev1.CheBackupServerConfiguration
	backupServerConfigName := restoreCR.Spec.BackupServerConfigRef
	if backupServerConfigName == "" {
		// Try to find backup server configuration in the same namespace
		cheBackupServersConfigurationList := &chev1.CheBackupServerConfigurationList{}
		listOptions := &client.ListOptions{Namespace: restoreCR.GetNamespace()}
		if err := r.cachingClient.List(context.TODO(), cheBackupServersConfigurationList, listOptions); err != nil {
			return false, err
		}
		if len(cheBackupServersConfigurationList.Items) != 1 {
			return true, fmt.Errorf("expected an instance of CheBackupServersConfiguration, but got %d instances", len(cheBackupServersConfigurationList.Items))
		}
		backupServerConfigName = cheBackupServersConfigurationList.Items[0].GetName()
	}
	backupServerConfigCR = &chev1.CheBackupServerConfiguration{}
	backupServerConfigNamespacedName := types.NamespacedName{Namespace: restoreCR.GetNamespace(), Name: backupServerConfigName}
	if err := r.cachingClient.Get(context.TODO(), backupServerConfigNamespacedName, backupServerConfigCR); err != nil {
		if errors.IsNotFound(err) {
			return true, fmt.Errorf("backup server configuration with name '%s' not found in '%s' namespace", restoreCR.Spec.BackupServerConfigRef, restoreCR.GetNamespace())
		}
		return false, err
	}

	rctx, err := NewRestoreContext(r, restoreCR, backupServerConfigCR)
	if err != nil {
		// Failed to create context.
		// This is usually caused by invalid configuration of current backup server in the restore CR.
		// Do not requeue as user has to correct the configuration manually.
		return true, err
	}

	// Update status with progress on the first reconcile loop
	if rctx.restoreCR.Status.State == "" {
		rctx.restoreCR.Status.Message = "Restore is in progress. Start time: " + time.Now().String()
		rctx.restoreCR.Status.State = chev1.STATE_IN_PROGRESS
		rctx.restoreCR.Status.Phase = rctx.state.GetPhaseMessage()
		if err := r.UpdateCRStatus(restoreCR); err != nil {
			return false, err
		}
	}

	// Make sure, that backup server configuration in the CR is valid and cache cluster resources
	done, err = rctx.backupServer.PrepareConfiguration(rctx.r.nonCachingClient, rctx.namespace)
	if err != nil || !done {
		return done, err
	}

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
		rctx.UpdateRestoreStatus()
	}

	if !rctx.state.cheRestored {
		// Restore all data from the backup
		done, err = RestoreChe(rctx, backupDataDestDir)
		if err != nil || !done {
			return done, err
		}

		// Clean up backup data after successful restore
		if err := os.RemoveAll(backupDataDestDir); err != nil {
			return false, err
		}

		rctx.state.cheRestored = true
	}

	rctx.restoreCR.Status.Message = "Restore successfully finished"
	rctx.restoreCR.Status.State = chev1.STATE_SUCCEEDED
	rctx.restoreCR.Status.Phase = ""
	if err := rctx.r.UpdateCRStatus(rctx.restoreCR); err != nil {
		logrus.Errorf("Failed to update status after successful restore: %v", err)
		return false, err
	}

	logrus.Info(rctx.restoreCR.Status.Message)
	return true, nil
}

func (r *ReconcileCheClusterRestore) UpdateCR(cr *chev1.CheClusterRestore) error {
	err := r.cachingClient.Update(context.TODO(), cr)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", cr.Name, err.Error())
		return err
	}
	return nil
}

func (r *ReconcileCheClusterRestore) UpdateCRStatus(cr *chev1.CheClusterRestore) error {
	err := r.cachingClient.Status().Update(context.TODO(), cr)
	if err != nil {
		logrus.Errorf("Failed to update %s CR status: %s", cr.Name, err.Error())
		return err
	}
	logrus.Infof("Status updated with %v: ", cr.Status)
	return nil
}
