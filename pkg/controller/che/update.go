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

	identity_provider "github.com/eclipse/che-operator/pkg/deploy/identity-provider"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileChe) UpdateCheCRStatus(instance *orgv1.CheCluster, updatedField string, value string) (err error) {
	logrus.Infof("Updating %s CR with %s: %s", instance.Name, updatedField, value)
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		logrus.Errorf("Failed to update %s CR. Fetching the latest CR version: %s", instance.Name, err)
		return err
	}
	logrus.Infof("Custom resource %s updated", instance.Name)
	return nil
}

func (r *ReconcileChe) UpdateCheCRSpec(instance *orgv1.CheCluster, updatedField string, value string) (err error) {
	logrus.Infof("Updating %s CR with %s: %s", instance.Name, updatedField, value)
	err = r.client.Update(context.TODO(), instance)
	if err != nil {
		logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
		return err
	}
	logrus.Infof("Custom resource %s updated", instance.Name)
	return nil
}

func (r *ReconcileChe) ReconcileIdentityProvider(instance *orgv1.CheCluster, isOpenShift4 bool) (deleted bool, err error) {
	if !util.IsOAuthEnabled(instance) && instance.Status.OpenShiftoAuthProvisioned == true {
		keycloakDeployment := &appsv1.Deployment{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "keycloak", Namespace: instance.Namespace}, keycloakDeployment); err != nil {
			logrus.Errorf("Deployment %s not found: %s", keycloakDeployment.Name, err)
		}

		providerName := "openshift-v3"
		if isOpenShift4 {
			providerName = "openshift-v4"
		}
		_, err := util.K8sclient.ExecIntoPod(
			instance,
			keycloakDeployment.Name,
			func(cr *orgv1.CheCluster) (string, error) {
				return identity_provider.GetDeleteIdentityProviderCommand(instance, providerName)
			},
			"delete OpenShift identity provider")
		if err == nil {
			oAuthClient := &oauth.OAuthClient{}
			oAuthClientName := instance.Spec.Auth.OAuthClientName
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName, Namespace: ""}, oAuthClient); err != nil {
				logrus.Errorf("OAuthClient %s not found: %s", oAuthClient.Name, err)
			}
			if err := r.client.Delete(context.TODO(), oAuthClient); err != nil {
				logrus.Errorf("Failed to delete %s %s: %s", oAuthClient.Kind, oAuthClient.Name, err)
			}
			return true, nil
		}
		return false, err
	}
	return false, nil
}
