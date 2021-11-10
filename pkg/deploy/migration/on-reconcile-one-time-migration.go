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

package migration

import (
	"context"
	"time"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	gitSelfSignedCertsConfigMapName = "che-git-self-signed-cert"
)

type Migrator struct {
	deploy.Reconcilable

	migrationDone bool
}

func NewMigrator() *Migrator {
	return &Migrator{
		migrationDone: false,
	}
}

func (m *Migrator) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if m.migrationDone {
		return reconcile.Result{}, true, nil
	}

	result, done, err := m.migrate(ctx)
	if done && err == nil {
		m.migrationDone = true
	}
	return result, done, err
}

func (m *Migrator) migrate(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	// Add required labels for the config map with additional CA certificates
	if ctx.CheCluster.Spec.Server.ServerTrustStoreConfigMapName != "" {
		if err := addRequiredLabelsForConfigMap(ctx, ctx.CheCluster.Spec.Server.ServerTrustStoreConfigMapName, deploy.CheCACertsConfigMapLabelValue); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	// Add required labels for the config map with CA certificates for git
	if ctx.CheCluster.Spec.Server.GitSelfSignedCert {
		if err := addRequiredLabelsForConfigMap(ctx, gitSelfSignedCertsConfigMapName, deploy.CheCACertsConfigMapLabelValue); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	// Give some time for the migration resources to be flushed
	return reconcile.Result{RequeueAfter: 5 * time.Second}, true, nil
}

func addRequiredLabelsForConfigMap(ctx *deploy.DeployContext, configMapName string, componentName string) error {
	configMap := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: configMapName}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if configMap.ObjectMeta.Labels == nil {
		configMap.ObjectMeta.Labels = make(map[string]string)
	}
	for labelName, labelValue := range deploy.GetLabels(ctx.CheCluster, componentName) {
		configMap.ObjectMeta.Labels[labelName] = labelValue
	}
	if err := ctx.ClusterAPI.NonCachingClient.Update(context.TODO(), configMap); err != nil {
		return err
	}

	return nil
}
