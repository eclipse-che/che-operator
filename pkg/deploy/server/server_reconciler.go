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

package server

import (
	"fmt"
	"time"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	crb                   = ".crb."
	cheCRBFinalizerSuffix = crb + constants.FinalizerSuffix
	configMapName         = "che"
)

var log = ctrl.Log.WithName("server")

type CheServerReconciler struct {
	reconciler.Reconcilable
}

func NewCheServerReconciler() *CheServerReconciler {
	return &CheServerReconciler{}
}

func (s *CheServerReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	done, err := s.syncConfigMap(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	// ensure configmap is created
	// the version of the object is used in the deployment
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: configMapName, Namespace: cheCtx.CheCluster.Namespace},
		&corev1.ConfigMap{},
	)
	if !exists {
		return reconcile.Result{}, false, err
	}

	if err := deploy.SyncServiceAccountToCluster(cheCtx, constants.DefaultCheServiceAccountName); err != nil {
		return reconcile.Result{}, false, err
	}

	if done, err := s.syncPermissions(cheCtx); !done {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	done, err = s.syncDeployment(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = s.syncActiveChePhase(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = s.syncCheVersion(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = s.syncCheURL(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (c *CheServerReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	return c.deletePermissions(cheCtx)
}

func (s *CheServerReconciler) syncActiveChePhase(cheCtx *chetypes.CheContext) (bool, error) {
	cheDeployment := &appsv1.Deployment{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: getComponentName(), Namespace: cheCtx.CheCluster.Namespace},
		cheDeployment,
	)
	if err != nil {
		return false, fmt.Errorf("failed to get Deployment %s/%s: %w", cheCtx.CheCluster.Namespace, getComponentName(), err)
	}

	if exists {
		if cheDeployment.Status.AvailableReplicas == 0 {
			if cheCtx.CheCluster.Status.ChePhase != chev2.ClusterPhaseInactive {
				cheCtx.CheCluster.Status.ChePhase = chev2.ClusterPhaseInactive
				err := deploy.UpdateCheCRStatus(cheCtx, "Phase", chev2.ClusterPhaseInactive)
				return false, err
			}
		} else if cheDeployment.Status.Replicas != cheDeployment.Status.AvailableReplicas {
			if cheCtx.CheCluster.Status.ChePhase != chev2.RollingUpdate {
				cheCtx.CheCluster.Status.ChePhase = chev2.RollingUpdate
				err := deploy.UpdateCheCRStatus(cheCtx, "Phase", chev2.RollingUpdate)
				return false, err
			}
		} else {
			if cheCtx.CheCluster.Status.ChePhase != chev2.ClusterPhaseActive {
				cheCtx.CheCluster.Status.ChePhase = chev2.ClusterPhaseActive
				err := deploy.UpdateCheCRStatus(cheCtx, "Phase", chev2.ClusterPhaseActive)
				return err == nil, err
			}
		}
	} else {
		cheCtx.CheCluster.Status.ChePhase = chev2.ClusterPhaseInactive
		err := deploy.UpdateCheCRStatus(cheCtx, "Phase", chev2.ClusterPhaseInactive)
		return false, err
	}

	return true, nil
}

func (s *CheServerReconciler) getCRBFinalizerName(crbName string) string {
	finalizer := crbName + cheCRBFinalizerSuffix
	diff := len(finalizer) - 63
	if diff > 0 {
		return finalizer[:len(finalizer)-diff]
	}
	return finalizer
}

func (s *CheServerReconciler) syncDeployment(cheCtx *chetypes.CheContext) (bool, error) {
	spec, err := s.getDeploymentSpec(cheCtx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(cheCtx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (s CheServerReconciler) syncCheVersion(cheCtx *chetypes.CheContext) (bool, error) {
	cheVersion := defaults.GetCheVersion()
	if cheCtx.CheCluster.Status.CheVersion != cheVersion {
		cheCtx.CheCluster.Status.CheVersion = cheVersion
		err := deploy.UpdateCheCRStatus(cheCtx, "version", cheVersion)
		return err == nil, err
	}
	return true, nil
}

func (s CheServerReconciler) syncCheURL(cheCtx *chetypes.CheContext) (bool, error) {
	var cheUrl = "https://" + cheCtx.CheHost
	if cheCtx.CheCluster.Status.CheURL != cheUrl {
		product := map[bool]string{true: "Red Hat OpenShift Dev Spaces", false: "Eclipse Che"}[defaults.GetCheFlavor() == "devspaces"]
		logrus.Infof("%s is now available at: %s", product, cheUrl)

		cheCtx.CheCluster.Status.CheURL = cheUrl
		err := deploy.UpdateCheCRStatus(cheCtx, getComponentName()+" server URL", cheUrl)
		return err == nil, err
	}

	return true, nil
}
