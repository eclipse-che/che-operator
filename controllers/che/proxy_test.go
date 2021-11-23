//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"os"
	"reflect"
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestReadProxyConfiguration(t *testing.T) {
	type testCase struct {
		name              string
		openShiftVersion  string
		cheCluster        *orgv1.CheCluster
		clusterProxy      *configv1.Proxy
		initObjects       []runtime.Object
		expectedProxyConf *deploy.Proxy
	}

	testCases := []testCase{
		{
			name:             "Test no proxy configured",
			openShiftVersion: "4",
			clusterProxy: &configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
			initObjects:       []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{},
		},
		{
			name:             "Test checluster proxy configured, OpenShift 4.x",
			openShiftVersion: "4",
			clusterProxy: &configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ProxyURL:      "http://proxy",
						ProxyPort:     "3128",
						NonProxyHosts: "host1",
					},
				},
			},
			initObjects: []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{
				HttpProxy:        "http://proxy:3128",
				HttpUser:         "",
				HttpPassword:     "",
				HttpHost:         "proxy",
				HttpPort:         "3128",
				HttpsProxy:       "http://proxy:3128",
				HttpsUser:        "",
				HttpsPassword:    "",
				HttpsHost:        "proxy",
				HttpsPort:        "3128",
				NoProxy:          "host1,.svc",
				TrustedCAMapName: "",
			},
		},
		{
			name:             "Test checluster proxy configured, nonProxy merged, OpenShift 4.x",
			openShiftVersion: "4",
			clusterProxy: &configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: configv1.ProxyStatus{
					HTTPProxy:  "http://proxy:3128",
					HTTPSProxy: "http://proxy:3128",
					NoProxy:    "host2",
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ProxyURL:      "http://proxy",
						ProxyPort:     "3128",
						NonProxyHosts: "host1",
					},
				},
			},
			initObjects: []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{
				HttpProxy:        "http://proxy:3128",
				HttpUser:         "",
				HttpPassword:     "",
				HttpHost:         "proxy",
				HttpPort:         "3128",
				HttpsProxy:       "http://proxy:3128",
				HttpsUser:        "",
				HttpsPassword:    "",
				HttpsHost:        "proxy",
				HttpsPort:        "3128",
				NoProxy:          "host1,host2",
				TrustedCAMapName: "",
			},
		},
		{
			name:             "Test cluster wide proxy configured, OpenShift 4.x",
			openShiftVersion: "4",
			clusterProxy: &configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.ProxySpec{
					TrustedCA: configv1.ConfigMapNameReference{
						Name: "additional-cluster-ca-bundle",
					},
				},
				Status: configv1.ProxyStatus{
					HTTPProxy:  "http://proxy:3128",
					HTTPSProxy: "http://proxy:3128",
					NoProxy:    "host1",
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{},
				},
			},
			initObjects: []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{
				HttpProxy:        "http://proxy:3128",
				HttpUser:         "",
				HttpPassword:     "",
				HttpHost:         "proxy",
				HttpPort:         "3128",
				HttpsProxy:       "http://proxy:3128",
				HttpsUser:        "",
				HttpsPassword:    "",
				HttpsHost:        "proxy",
				HttpsPort:        "3128",
				NoProxy:          "host1",
				TrustedCAMapName: "additional-cluster-ca-bundle",
			},
		},
		{
			name:             "Test cluster wide proxy is not configured, but cluster wide CA certs added, OpenShift 4.x",
			openShiftVersion: "4",
			clusterProxy: &configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.ProxySpec{
					TrustedCA: configv1.ConfigMapNameReference{
						Name: "additional-cluster-ca-bundle",
					},
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
			initObjects: []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{
				TrustedCAMapName: "additional-cluster-ca-bundle",
			},
		},
		{
			name:             "Test cluster wide proxy configured, nonProxy merged, OpenShift 4.x",
			openShiftVersion: "4",
			clusterProxy: &configv1.Proxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Status: configv1.ProxyStatus{
					HTTPProxy:  "http://proxy:3128",
					HTTPSProxy: "http://proxy:3128",
					NoProxy:    "host1",
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						NonProxyHosts: "host2",
					},
				},
			},
			initObjects: []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{
				HttpProxy:        "http://proxy:3128",
				HttpUser:         "",
				HttpPassword:     "",
				HttpHost:         "proxy",
				HttpPort:         "3128",
				HttpsProxy:       "http://proxy:3128",
				HttpsUser:        "",
				HttpsPassword:    "",
				HttpsHost:        "proxy",
				HttpsPort:        "3128",
				NoProxy:          "host1,host2",
				TrustedCAMapName: "",
			},
		},
		{
			name:             "Test checluster proxy configured, OpenShift 3.x",
			openShiftVersion: "3",
			clusterProxy:     &configv1.Proxy{},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
						ProxyURL:                       "http://proxy",
						ProxyPort:                      "3128",
						NonProxyHosts:                  "host1",
					},
				},
			},
			initObjects: []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{
				HttpProxy:        "http://proxy:3128",
				HttpUser:         "",
				HttpPassword:     "",
				HttpHost:         "proxy",
				HttpPort:         "3128",
				HttpsProxy:       "http://proxy:3128",
				HttpsUser:        "",
				HttpsPassword:    "",
				HttpsHost:        "proxy",
				HttpsPort:        "3128",
				NoProxy:          "host1",
				TrustedCAMapName: "",
			},
		},
		{
			name:             "Test checluster proxy configured, OpenShift 3.x and k8s, svc usage",
			openShiftVersion: "3",
			clusterProxy:     &configv1.Proxy{},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ProxyURL:      "http://proxy",
						ProxyPort:     "3128",
						NonProxyHosts: "host1",
					},
				},
			},
			initObjects: []runtime.Object{},
			expectedProxyConf: &deploy.Proxy{
				HttpProxy:        "http://proxy:3128",
				HttpUser:         "",
				HttpPassword:     "",
				HttpHost:         "proxy",
				HttpPort:         "3128",
				HttpsProxy:       "http://proxy:3128",
				HttpsUser:        "",
				HttpsPassword:    "",
				HttpsHost:        "proxy",
				HttpsPort:        "3128",
				NoProxy:          "host1,.svc",
				TrustedCAMapName: "",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.clusterProxy, testCase.cheCluster)

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.Proxy{})

			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, _ := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			os.Setenv("OPENSHIFT_VERSION", testCase.openShiftVersion)
			util.IsOpenShift, util.IsOpenShift4, _ = util.DetectOpenShift()

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client:           cli,
					NonCachingClient: cli,
					Scheme:           scheme,
				},
			}

			actualProxyConf, err := GetProxyConfiguration(deployContext)
			if err != nil {
				t.Fatalf("Error reading proxy configuration: %v", err)
			}

			if !reflect.DeepEqual(testCase.expectedProxyConf, actualProxyConf) {
				t.Errorf("Expected deployment and deployment returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedProxyConf, actualProxyConf))
			}
		})
	}
}
