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
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const openVSXPathPrefix = "/openvsx"

type OpenVSXServerReconciler struct {
	reconciler.Reconcilable
}

func NewOpenVSXServerReconciler() *OpenVSXServerReconciler {
	return &OpenVSXServerReconciler{}
}

func (r *OpenVSXServerReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsOpenVSXOperandEnabled() {
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXServerName, &appsv1.Deployment{})
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXServerName, &corev1.Service{})
		_, _ = deploy.DeleteNamespacedObject(ctx, configMapName, &corev1.ConfigMap{})
		_, _ = deploy.DeleteNamespacedObject(ctx, gateway.GatewayConfigMapNamePrefix+constants.OpenVSXServerName, &corev1.ConfigMap{})

		if ctx.CheCluster.Status.OpenVSXURL != "" {
			ctx.CheCluster.Status.OpenVSXURL = ""
			err := deploy.UpdateCheCRStatus(ctx, "OpenVSXURL", "")
			return reconcile.Result{}, err == nil, err
		}

		return reconcile.Result{}, true, nil
	}

	done, err := r.syncService(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	endpoint, done, err := r.exposeEndpoint(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.updateStatus(endpoint, ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncConfigMap(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = r.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXServerReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (r *OpenVSXServerReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		constants.OpenVSXServerName,
		[]string{"http"},
		[]int32{8080},
		constants.OpenVSXServerName)
}

func (r *OpenVSXServerReconciler) exposeEndpoint(ctx *chetypes.DeployContext) (string, bool, error) {
	return expose.ExposeWithHostPath(
		ctx,
		constants.OpenVSXServerName,
		"",
		openVSXPathPrefix,
		r.createGatewayConfig())
}

func (r *OpenVSXServerReconciler) updateStatus(endpoint string, ctx *chetypes.DeployContext) (bool, error) {
	openVSXURL := "https://" + endpoint

	if openVSXURL != ctx.CheCluster.Status.OpenVSXURL {
		ctx.CheCluster.Status.OpenVSXURL = openVSXURL
		if err := deploy.UpdateCheCRStatus(ctx, "status: OpenVSX URL", openVSXURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (r *OpenVSXServerReconciler) syncConfigMap(ctx *chetypes.DeployContext) (bool, error) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXServerName),
		},
		Data: map[string]string{
			"application.yml": applicationConfig,
		},
	}

	return deploy.Sync(ctx, cm, diffs.ConfigMapAllLabels)
}

func (r *OpenVSXServerReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := r.getDeploymentSpec(ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (r *OpenVSXServerReconciler) createGatewayConfig() *gateway.TraefikConfig {
	cfg := gateway.CreateCommonTraefikConfig(
		constants.OpenVSXServerName,
		"PathPrefix(`/openvsx/api`) || "+
			"PathPrefix(`/openvsx/user`) || "+
			"PathPrefix(`/openvsx/login`) || "+
			"PathPrefix(`/openvsx/logout`) || "+
			"PathPrefix(`/openvsx/oauth2`) || "+
			"PathPrefix(`/openvsx/login-providers`) || "+
			"PathPrefix(`/openvsx/admin`) || "+
			"PathPrefix(`/openvsx/actuator`) || "+
			"PathPrefix(`/openvsx/documents`) || "+
			"PathPrefix(`/openvsx/swagger-ui`) || "+
			"PathPrefix(`/openvsx/v3`)",
		20,
		"http://"+constants.OpenVSXServerName+":8080",
		[]string{openVSXPathPrefix})

	return cfg
}
