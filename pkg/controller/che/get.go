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
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileChe) GetEffectiveDeployment(instance *orgv1.CheCluster, name string) (deployment *appsv1.Deployment, err error) {
	deployment = &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, deployment)
	return deployment, err
}

func (r *ReconcileChe) GetCR(request reconcile.Request) (instance *orgv1.CheCluster, err error) {
	instance = &orgv1.CheCluster{}
	err = r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		logrus.Errorf("Failed to get %s CR: %s", instance.Name, err)
		return nil, err
	}
	return instance, nil
}

func (r *ReconcileChe) GetOAuthClient(oAuthClientName string) (oAuthClient *oauth.OAuthClient, err error) {
	oAuthClient = &oauth.OAuthClient{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName, Namespace: ""}, oAuthClient); err != nil {
		logrus.Errorf("Failed to Get oAuthClient %s: %s", oAuthClientName, err)
		return nil, err
	}
	return oAuthClient, nil
}
