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
	"k8s.io/apimachinery/pkg/api/errors"
	"context"
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileChe) GetEffectiveDeployment(instance *orgv1.CheCluster, name string) (deployment *appsv1.Deployment, err error) {
	deployment = &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, deployment)
	if err != nil {
		logrus.Errorf("Failed to get %s deployment: %s", name, err)
		return nil, err
	}
	return deployment, nil
}

func (r *ReconcileChe) GetEffectiveIngress(instance *orgv1.CheCluster, name string) (ingress *v1beta1.Ingress) {
	ingress = &v1beta1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, ingress)
	if err != nil {
		logrus.Errorf("Failed to get %s ingress: %s", name, err)
		return nil
	}
	return ingress
}

func (r *ReconcileChe) GetEffectiveRoute(instance *orgv1.CheCluster, name string) (route *routev1.Route) {
	route = &routev1.Route{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, route)
	if err != nil {
		logrus.Errorf("Failed to get %s route: %s", name, err)
		return nil
	}
	return route
}

func (r *ReconcileChe) GetEffectiveConfigMap(instance *orgv1.CheCluster, name string) (configMap *corev1.ConfigMap) {
	configMap = &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, configMap)
	if err != nil {
		logrus.Errorf("Failed to get %s config map: %s", name, err)
		return nil
	}
	return configMap

}

func (r *ReconcileChe) GetEffectiveSecretResourceVersion(instance *orgv1.CheCluster, name string) string {
	secret := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, secret)
	if err != nil {
		if !errors.IsNotFound(err){
			logrus.Errorf("Failed to get %s secret: %s", name, err)
		}
		return ""
	}
	return secret.ResourceVersion
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

func (r *ReconcileChe)GetDeploymentVolume(deployment *appsv1.Deployment, key string) (volume corev1.Volume) {
	volumes := deployment.Spec.Template.Spec.Volumes
	for i := range volumes {
		name := volumes[i].Name
		if name == key {
			volume = volumes[i]
			break
		}
	}
	return volume
}