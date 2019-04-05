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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
	clientSet, err     = getClientSet()
	oauthClientSet, _  = getOAuthClientSet()
	client             = GetK8Config()
	SchemeGroupVersion = schema.GroupVersion{Group: groupName, Version: orgv1.SchemeGroupVersion.Version}
)

type k8s struct {
	clientset kubernetes.Interface
}

type CRClient struct {
	restClient rest.Interface
}

type OauthClient struct {
	restClient rest.Interface
}

func GetK8Config() *k8s {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf(err.Error())
	}
	client := k8s{}
	client.clientset, err = kubernetes.NewForConfig(cfg)

	if err != nil {
		logrus.Errorf(err.Error())
	}
	return &client
}

func getClientSet() (clientSet *CRClient, err error) {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf(err.Error())
	}
	client := k8s{}
	client.clientset, err = kubernetes.NewForConfig(cfg)
	clientSet, err = newForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func getOAuthClientSet() (clientSet *OauthClient, err error) {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Errorf(err.Error())
	}
	client := k8s{}
	client.clientset, err = kubernetes.NewForConfig(cfg)
	clientSet, err = newOAuthConfig(cfg)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func getCR() (*orgv1.CheCluster, error) {

	result := orgv1.CheCluster{}
	opts := metav1.ListOptions{}
	err = clientSet.restClient.
		Get().
		Namespace(namespace).
		Resource(kind).
		Name(crName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func newForConfig(c *rest.Config) (*CRClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: groupName, Version: orgv1.SchemeGroupVersion.Version}
	//config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: oauth.GroupName, Version: oauth.SchemeGroupVersion.Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &CRClient{restClient: client}, nil
}

func newOAuthConfig(c *rest.Config) (*OauthClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: oauth.GroupName, Version: oauth.SchemeGroupVersion.Version}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: scheme.Codecs}
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &OauthClient{restClient: client}, nil
}

func addKnownTypes(scheme *runtime.Scheme) (error) {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&orgv1.CheCluster{},
		&orgv1.CheClusterList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
