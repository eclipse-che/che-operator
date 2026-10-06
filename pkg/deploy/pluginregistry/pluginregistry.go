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

package pluginregistry

import (
	"fmt"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
)

type PluginRegistryReconciler struct {
	reconciler.Reconcilable
}

func NewPluginRegistryReconciler() *PluginRegistryReconciler {
	return &PluginRegistryReconciler{}
}

func (p *PluginRegistryReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if cheCtx.CheCluster.IsInternalPluginRegistryDisabled() {
		objects := []struct {
			name string
			obj  client.Object
		}{
			{constants.PluginRegistryName, &corev1.Service{}},
			{constants.PluginRegistryName, &corev1.ConfigMap{}},
			{gateway.GatewayConfigMapNamePrefix + constants.PluginRegistryName, &corev1.ConfigMap{}},
			{constants.PluginRegistryName, &appsv1.Deployment{}},
		}

		for _, object := range objects {
			key := types.NamespacedName{Name: object.name, Namespace: cheCtx.CheCluster.Namespace}
			_ = cheCtx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(cheCtx.Context, key, object.obj)
		}

		if cheCtx.CheCluster.Status.PluginRegistryURL != "" {
			cheCtx.CheCluster.Status.PluginRegistryURL = ""
			err := deploy.UpdateCheCRStatus(cheCtx, "PluginRegistryURL", "")
			return reconcile.Result{}, err == nil, err
		}

		return reconcile.Result{}, true, nil
	}

	if err := p.syncService(cheCtx); err != nil {
		return reconcile.Result{}, false, err
	}

	endpoint, done, err := p.ExposeEndpoint(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.updateStatus(endpoint, cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	if err := p.syncConfigMap(cheCtx); err != nil {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncDeployment(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (p *PluginRegistryReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	return true
}

func (p *PluginRegistryReconciler) syncService(cheCtx *chetypes.CheContext) error {
	return deploy.SyncServiceToCluster(
		cheCtx,
		constants.PluginRegistryName,
		[]string{"http"},
		[]int32{8080},
		constants.PluginRegistryName)
}

func (p *PluginRegistryReconciler) syncConfigMap(cheCtx *chetypes.CheContext) error {
	data, err := p.getConfigMapData(cheCtx)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap data: %w", err)
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.PluginRegistryName,
			Namespace:   cheCtx.CheCluster.Namespace,
			Labels:      deploy.GetLabels(constants.PluginRegistryName),
			Annotations: data,
		},
		Data: data,
	}

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, cm, cheCtx.ClusterAPI.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
	}

	if err := cheCtx.ClusterAPI.ClientWrapper.Sync(
		cheCtx.Context,
		cm,
		&k8sclient.SyncOptions{DiffOpts: diffs.ConfigMapEnsureLabels},
	); err != nil {
		return fmt.Errorf("failed to sync ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
	}

	return nil
}

func (p *PluginRegistryReconciler) ExposeEndpoint(cheCtx *chetypes.CheContext) (string, bool, error) {
	return expose.Expose(
		cheCtx,
		constants.PluginRegistryName,
		p.createGatewayConfig(cheCtx))
}

func (p *PluginRegistryReconciler) updateStatus(endpoint string, cheCtx *chetypes.CheContext) (bool, error) {
	pluginRegistryURL := "https://" + endpoint

	// append the API version to plugin registry
	if !strings.HasSuffix(pluginRegistryURL, "/") {
		pluginRegistryURL = pluginRegistryURL + "/v3"
	} else {
		pluginRegistryURL = pluginRegistryURL + "v3"
	}

	if pluginRegistryURL != cheCtx.CheCluster.Status.PluginRegistryURL {
		cheCtx.CheCluster.Status.PluginRegistryURL = pluginRegistryURL
		if err := deploy.UpdateCheCRStatus(cheCtx, "status: Plugin Registry URL", pluginRegistryURL); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (p *PluginRegistryReconciler) syncDeployment(cheCtx *chetypes.CheContext) (bool, error) {
	if spec, err := p.getPluginRegistryDeploymentSpec(cheCtx); err != nil {
		return false, err
	} else {
		return deploy.SyncDeploymentSpecToCluster(cheCtx, spec, deploy.DefaultDeploymentDiffOpts)
	}
}

func (p *PluginRegistryReconciler) createGatewayConfig(cheCtx *chetypes.CheContext) *gateway.TraefikConfig {
	pathPrefix := "/" + constants.PluginRegistryName
	cfg := gateway.CreateCommonTraefikConfig(
		constants.PluginRegistryName,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		10,
		"http://"+constants.PluginRegistryName+":8080",
		[]string{pathPrefix})

	return cfg
}
