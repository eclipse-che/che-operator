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
package server

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	AvailableStatus               = "Available"
	UnavailableStatus             = "Unavailable"
	RollingUpdateInProgressStatus = "Available: Rolling update in progress"
)

type CheServerReconciler struct {
	deploy.Reconcilable
}

func NewCheServerReconciler() *CheServerReconciler {
	return &CheServerReconciler{}
}

func (s *CheServerReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	done, err := s.syncLegacyConfigMap(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = s.syncCheConfigMap(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	// ensure configmap is created
	// the version of the object is used in the deployment
	exists, err := deploy.GetNamespacedObject(ctx, CheConfigMapName, &corev1.ConfigMap{})
	if !exists {
		return reconcile.Result{}, false, err
	}

	done, err = s.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = s.updateAvailabilityStatus(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = s.updateCheURL(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = s.updateCheVersion(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (s *CheServerReconciler) Finalize(ctx *deploy.DeployContext) error {
	return nil
}

func (s CheServerReconciler) updateCheURL(ctx *deploy.DeployContext) (bool, error) {
	var cheUrl = util.GetCheURL(ctx.CheCluster)
	if ctx.CheCluster.Status.CheURL != cheUrl {
		ctx.CheCluster.Status.CheURL = cheUrl
		err := deploy.UpdateCheCRStatus(ctx, getComponentName(ctx)+" server URL", cheUrl)
		return err == nil, err
	}

	return true, nil
}

func (s *CheServerReconciler) syncCheConfigMap(ctx *deploy.DeployContext) (bool, error) {
	data, err := s.getCheConfigMapData(ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncConfigMapDataToCluster(ctx, CheConfigMapName, data, getComponentName(ctx))
}

func (s CheServerReconciler) syncLegacyConfigMap(ctx *deploy.DeployContext) (bool, error) {
	// Get custom ConfigMap
	// if it exists, add the data into CustomCheProperties
	customConfigMap := &corev1.ConfigMap{}
	exists, err := deploy.GetNamespacedObject(ctx, "custom", customConfigMap)
	if err != nil {
		return false, err
	} else if exists {
		logrus.Info("Found legacy custom ConfigMap. Adding those values to CheCluster.Spec.Server.CustomCheProperties")

		if ctx.CheCluster.Spec.Server.CustomCheProperties == nil {
			ctx.CheCluster.Spec.Server.CustomCheProperties = make(map[string]string)
		}
		for k, v := range customConfigMap.Data {
			ctx.CheCluster.Spec.Server.CustomCheProperties[k] = v
		}

		err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
		if err != nil {
			return false, err
		}

		return deploy.DeleteNamespacedObject(ctx, "custom", &corev1.ConfigMap{})
	}

	return true, nil
}

func (s *CheServerReconciler) updateAvailabilityStatus(ctx *deploy.DeployContext) (bool, error) {
	cheDeployment := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(ctx, getComponentName(ctx), cheDeployment)
	if err != nil {
		return false, err
	}

	if exists {
		if cheDeployment.Status.AvailableReplicas < 1 {
			if ctx.CheCluster.Status.CheClusterRunning != UnavailableStatus {
				ctx.CheCluster.Status.CheClusterRunning = UnavailableStatus
				err := deploy.UpdateCheCRStatus(ctx, "status: Che API", UnavailableStatus)
				return err == nil, err
			}
		} else if cheDeployment.Status.Replicas != 1 {
			if ctx.CheCluster.Status.CheClusterRunning != RollingUpdateInProgressStatus {
				ctx.CheCluster.Status.CheClusterRunning = RollingUpdateInProgressStatus
				err := deploy.UpdateCheCRStatus(ctx, "status: Che API", RollingUpdateInProgressStatus)
				return err == nil, err
			}
		} else {
			if ctx.CheCluster.Status.CheClusterRunning != AvailableStatus {
				cheFlavor := deploy.DefaultCheFlavor(ctx.CheCluster)
				name := "Eclipse Che"
				if cheFlavor == "codeready" {
					name = "CodeReady Workspaces"
				}

				logrus.Infof(name+" is now available at: %s", util.GetCheURL(ctx.CheCluster))
				ctx.CheCluster.Status.CheClusterRunning = AvailableStatus
				err := deploy.UpdateCheCRStatus(ctx, "status: Che API", AvailableStatus)
				return err == nil, err
			}
		}
	} else {
		ctx.CheCluster.Status.CheClusterRunning = UnavailableStatus
		err := deploy.UpdateCheCRStatus(ctx, "status: Che API", UnavailableStatus)
		return err == nil, err
	}

	return true, nil
}

func (s *CheServerReconciler) syncDeployment(ctx *deploy.DeployContext) (bool, error) {
	spec, err := s.getDeploymentSpec(ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (s CheServerReconciler) updateCheVersion(ctx *deploy.DeployContext) (bool, error) {
	cheVersion := deploy.DefaultCheVersion()
	if ctx.CheCluster.Status.CheVersion != cheVersion {
		ctx.CheCluster.Status.CheVersion = cheVersion
		err := deploy.UpdateCheCRStatus(ctx, "version", cheVersion)
		return err == nil, err
	}
	return true, nil
}
