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

package fakeclusterapi

import (
	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/common/clusterapi"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	securityv1 "github.com/openshift/api/security/v1"
	templatev1 "github.com/openshift/api/template/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type Option func(*config)

type config struct {
	scheme             *runtime.Scheme
	statusSubresources []client.Object
	discoveryResources []*metav1.APIResourceList
}

func WithScheme(scheme *runtime.Scheme) Option {
	return func(c *config) { c.scheme = scheme }
}

func WithStatusSubresources(objs ...client.Object) Option {
	return func(c *config) { c.statusSubresources = objs }
}

func WithDiscoveryResources(resources ...*metav1.APIResourceList) Option {
	return func(c *config) { c.discoveryResources = resources }
}

func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controllerv1alpha1.AddToScheme(scheme))
	utilruntime.Must(chev1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(configv1.Install(scheme))
	utilruntime.Must(consolev1.Install(scheme))
	utilruntime.Must(oauthv1.Install(scheme))
	utilruntime.Must(projectv1.Install(scheme))
	utilruntime.Must(routev1.Install(scheme))
	utilruntime.Must(securityv1.Install(scheme))
	utilruntime.Must(templatev1.Install(scheme))

	return scheme
}

func Setup(opts ...Option) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	scheme := cfg.scheme
	if scheme == nil {
		scheme = NewScheme()
	}

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(cfg.statusSubresources) > 0 {
		builder = builder.WithStatusSubresource(cfg.statusSubresources...)
	}
	fakeClient := builder.Build()

	clientSet := fakeclientset.NewClientset()
	discoveryClient, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
	if !ok {
		panic("failed to setup ClusterAPI")
	}
	discoveryClient.Resources = cfg.discoveryResources

	clusterapi.Set(fakeClient, fakeClient, discoveryClient, scheme)
}
