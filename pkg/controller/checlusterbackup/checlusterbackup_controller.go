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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	err = c.Watch(&source.Kind{Type: &orgv1.CheClusterBackup{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner CheClusterBackup
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &orgv1.CheClusterBackup{},
	})
	if err != nil {
		return err
	}

	return nil
}

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
	logrus.Info("ReconcileCheClusterBackup") // TODO delete debug code

	// Fetch the CheClusterBackup instance
	backupCR := &orgv1.CheClusterBackup{}
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

	if err := r.CheckBackupSettings(backupCR); err != nil {
		// An error happened while processing CR data
		return reconcile.Result{}, err
	}

	if backupCR.Spec.TriggerNow {
		// Should create a backup
		if err := r.CreateBackup(backupCR); err != nil {
			// An error happened while creating backup
			return reconcile.Result{}, err
		}

		// Backup is successfull
		backupCR.Spec.TriggerNow = false
		backupCR.Status.Message = "Backup successfully finished"
		r.UpdateCR(backupCR)
	}

	// Job is done
	return reconcile.Result{}, nil
}

func (r *ReconcileCheClusterBackup) UpdateCR(cr *orgv1.CheClusterBackup) error {
	err := r.client.Update(context.TODO(), cr)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", cr.Name, err.Error())
		return err
	}
	logrus.Infof("Custom resource %s updated", cr.Name)
	return nil
}

func (r *ReconcileCheClusterBackup) GetCheCR(namespace string) (*orgv1.CheCluster, error) {
	cheClusters := &orgv1.CheClusterList{}
	if err := r.client.List(context.TODO(), cheClusters, &client.ListOptions{}); err != nil {
		return nil, err
	}

	if len(cheClusters.Items) != 1 {
		return nil, fmt.Errorf("expected an instance of CheCluster, but got %d instances", len(cheClusters.Items))
	}

	cheCR := &orgv1.CheCluster{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: cheClusters.Items[0].GetName()}
	err := r.client.Get(context.TODO(), namespacedName, cheCR)
	if err != nil {
		return nil, err
	}
	return cheCR, nil
}
