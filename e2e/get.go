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
package main

import (
	oauth "github.com/openshift/api/oauth/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
func getOauthClient(name string)(oAuthClient *oauth.OAuthClient, err error) {
	oAuthClient = &oauth.OAuthClient{}
	err = oauthClientSet.restClient.Get().Name(name).Resource("oauthclients").Do().Into(oAuthClient)
	if err != nil && errors.IsNotFound(err) {
		return nil, err
	}
	return oAuthClient,nil
}


func getConfigMap(cmName string) (cm *corev1.ConfigMap, err error) {

	cm, err = client.clientset.CoreV1().ConfigMaps(namespace).Get(cmName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil

}