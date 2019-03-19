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
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func(r *ReconcileChe) GetEffectiveDeployment(instance *orgv1.CheCluster, name string) (deployment *appsv1.Deployment, err error) {
	deployment = &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, deployment)
	if err != nil {
		logrus.Errorf("Failed to get %s deployment: %s", name, err)
		return nil, err
	}
	return deployment, nil
}


func(r *ReconcileChe) GetEffectiveIngress(instance *orgv1.CheCluster, name string) (ingress *v1beta1.Ingress) {
	ingress = &v1beta1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: instance.Namespace}, ingress)
	if err != nil {
		logrus.Errorf("Failed to get %s ingress: %s", name, err)
		return nil
	}
	return ingress
}



func(r *ReconcileChe) GetEffectiveRoute(instance *orgv1.CheCluster, name string) (route *routev1.Route) {
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
		logrus.Errorf("Failed to get %s route: %s", name, err)
		return nil
	}
	return configMap

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