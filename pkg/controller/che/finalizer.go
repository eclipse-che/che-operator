//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package che

import (
	"context"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileChe) ReconcileFinalizer(instance *orgv1.CheCluster) (err error) {
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(instance.ObjectMeta.Finalizers, oAuthFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, oAuthFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				return err
			}
		}
	} else {
		if util.ContainsString(instance.ObjectMeta.Finalizers, oAuthFinalizerName) {
			oAuthClientName := instance.Spec.Auth.OAuthClientName
			logrus.Infof("Custom resource %s is being deleted. Deleting oAuthClient %s first", instance.Name, oAuthClientName)
			oAuthClient, err := r.GetOAuthClient(oAuthClientName)
			if err == nil {
				if err := r.client.Delete(context.TODO(), oAuthClient); err != nil {
					logrus.Errorf("Failed to delete %s oAuthClient: %s", oAuthClientName, err)
					return err
				}
			} else if !errors.IsNotFound(err) {
				logrus.Errorf("Failed to get %s oAuthClient: %s", oAuthClientName, err)
				return err
			}
			instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, oAuthFinalizerName)
			logrus.Infof("Updating %s CR", instance.Name)

			if err := r.client.Update(context.Background(), instance); err != nil {
				logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
				return err
			}
		}
		return nil
	}
	return nil
}

func (r *ReconcileChe) DeleteFinalizer(instance *orgv1.CheCluster) (err error) {
	instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, oAuthFinalizerName)
	logrus.Infof("Removing OAuth finalizer on %s CR", instance.Name)
	if err := r.client.Update(context.Background(), instance); err != nil {
		logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
		return err
	}
	return nil
}

func (r *ReconcileChe) HasImagePullerFinalizer(instance *orgv1.CheCluster) bool {
	finalizers := instance.ObjectMeta.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == imagePullerFinalizerName {
			return true
		}
	}
	return false
}

func (r *ReconcileChe) ReconcileImagePullerFinalizer(instance *orgv1.CheCluster) (err error) {
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(instance.ObjectMeta.Finalizers, imagePullerFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, imagePullerFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				return err
			}
		}
	} else {
		if util.ContainsString(instance.ObjectMeta.Finalizers, imagePullerFinalizerName) {
			clusterServiceVersionName := deploy.DefaultKubernetesImagePullerOperatorCSV()
			logrus.Infof("Custom resource %s is being deleted. Deleting ClusterServiceVersion %s first", instance.Name, clusterServiceVersionName)
			clusterServiceVersion := &operatorsv1alpha1.ClusterServiceVersion{}
			err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: clusterServiceVersionName}, clusterServiceVersion)
			if err != nil {
				logrus.Errorf("Error getting ClusterServiceVersion: %v", err)
				return err
			}
			if err := r.client.Delete(context.TODO(), clusterServiceVersion); err != nil {
				logrus.Errorf("Failed to delete %s ClusterServiceVersion: %s", clusterServiceVersionName, err)
				return err
			}
			instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, imagePullerFinalizerName)
			logrus.Infof("Updating %s CR", instance.Name)

			if err := r.client.Update(context.Background(), instance); err != nil {
				logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
				return err
			}
		}
		return nil
	}
	return nil
}

func (r *ReconcileChe) DeleteImagePullerFinalizer(instance *orgv1.CheCluster) (err error) {
	instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, imagePullerFinalizerName)
	logrus.Infof("Removing image puller finalizer on %s CR", instance.Name)
	if err := r.client.Update(context.Background(), instance); err != nil {
		logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
		return err
	}
	return nil
}
