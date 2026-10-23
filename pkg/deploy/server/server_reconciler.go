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
	"github.com/eclipse-che/che-operator/pkg/common"
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
	configMapName = "che"
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
			cheCtx.CheCluster.Status.ChePhase = chev2.ClusterPhaseInactive
		} else if cheDeployment.Status.Replicas != cheDeployment.Status.AvailableReplicas {
			cheCtx.CheCluster.Status.ChePhase = chev2.RollingUpdate
		} else {
			cheCtx.CheCluster.Status.ChePhase = chev2.ClusterPhaseActive
		}
	} else {
		cheCtx.CheCluster.Status.ChePhase = chev2.ClusterPhaseInactive
	}

	err = common.CheCluster.UpdateStatus(cheCtx)
	return err == nil, err
}

func (s *CheServerReconciler) syncDeployment(cheCtx *chetypes.CheContext) (bool, error) {
	spec, err := s.getDeploymentSpec(cheCtx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(cheCtx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (s CheServerReconciler) syncCheVersion(cheCtx *chetypes.CheContext) (bool, error) {
	cheCtx.CheCluster.Status.CheVersion = defaults.GetCheVersion()
	err := common.CheCluster.UpdateStatus(cheCtx)
	return err == nil, err
}

func (s CheServerReconciler) syncCheURL(cheCtx *chetypes.CheContext) (bool, error) {
	expectedCheUrl := "https://" + cheCtx.CheHost
	if cheCtx.CheCluster.Status.CheURL != expectedCheUrl {
		product := map[bool]string{true: "Red Hat OpenShift Dev Spaces", false: "Eclipse Che"}[defaults.GetCheFlavor() == "devspaces"]
		logrus.Infof("%s is now available at: %s", product, expectedCheUrl)
	}

	cheCtx.CheCluster.Status.CheURL = expectedCheUrl
	err := common.CheCluster.UpdateStatus(cheCtx)
	return err == nil, err
}
