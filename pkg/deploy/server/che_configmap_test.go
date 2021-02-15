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
package server

import (
	"os"
	"strings"
	"testing"

	"github.com/eclipse/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
)

func TestNewCheConfigMap(t *testing.T) {

	// since all values are retrieved from CR or auto-generated
	// some of them are explicitly set for this test to avoid using fake kube
	// and creating a CR with all spec fields pre-populated
	cr := &orgv1.CheCluster{}
	cr.Spec.Server.CheHost = "myhostname.com"
	cr.Spec.Server.TlsSupport = true
	cr.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(true)
	deployContext := &deploy.DeployContext{
		CheCluster: cr,
		Proxy:      &deploy.Proxy{},
		ClusterAPI: deploy.ClusterAPI{},
	}
	cheEnv, _ := GetCheConfigMapData(deployContext)
	testCm, _ := deploy.GetSpecConfigMap(deployContext, CheConfigMapName, cheEnv, CheConfigMapName)
	identityProvider := testCm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"]
	_, isOpenshiftv4, _ := util.DetectOpenShift()
	protocol := strings.Split(testCm.Data["CHE_API"], "://")[0]
	expectedIdentityProvider := "openshift-v3"
	if isOpenshiftv4 {
		expectedIdentityProvider = "openshift-v4"
	}
	if identityProvider != expectedIdentityProvider {
		t.Errorf("Test failed. Expecting identity provider to be '%s' while got '%s'", expectedIdentityProvider, identityProvider)
	}
	if protocol != "https" {
		t.Errorf("Test failed. Expecting 'https' protocol, got '%s'", protocol)
	}
}

func TestConfigMapOverride(t *testing.T) {
	cr := &orgv1.CheCluster{}
	cr.Spec.Server.CheHost = "myhostname.com"
	cr.Spec.Server.TlsSupport = true
	cr.Spec.Server.CustomCheProperties = map[string]string{
		"CHE_WORKSPACE_NO_PROXY": "myproxy.myhostname.com",
	}
	cr.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(true)
	deployContext := &deploy.DeployContext{
		CheCluster: cr,
		Proxy:      &deploy.Proxy{},
		ClusterAPI: deploy.ClusterAPI{},
	}
	cheEnv, _ := GetCheConfigMapData(deployContext)
	testCm, _ := deploy.GetSpecConfigMap(deployContext, CheConfigMapName, cheEnv, CheConfigMapName)
	if testCm.Data["CHE_WORKSPACE_NO_PROXY"] != "myproxy.myhostname.com" {
		t.Errorf("Test failed. Expected myproxy.myhostname.com but was %s", testCm.Data["CHE_WORKSPACE_NO_PROXY"])
	}
}

func TestConfigMap(t *testing.T) {
	type testCase struct {
		name         string
		isOpenShift  bool
		isOpenShift4 bool
		initObjects  []runtime.Object
		cheCluster   *orgv1.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name:         "Test k8s data, no tls secret",
			isOpenShift:  false,
			isOpenShift4: false,
			initObjects:  []runtime.Object{},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						WorkspaceNamespaceDefault: "<username>-che",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_KUBERNETES_TLS__CERT": "",
				"CHE_INFRA_KUBERNETES_TLS__KEY":  "",
			},
		},
		{
			name:         "Test k8s data, with tls secret",
			isOpenShift:  false,
			isOpenShift4: false,
			initObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "che-tls",
						Namespace: "eclipse-che",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("CRT"),
						"tls.key": []byte("KEY"),
					},
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						WorkspaceNamespaceDefault: "<username>-che",
					},
					K8s: orgv1.CheClusterSpecK8SOnly{
						TlsSecretName: "che-tls",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_KUBERNETES_TLS__CERT": "CRT",
				"CHE_INFRA_KUBERNETES_TLS__KEY":  "KEY",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			actualData, err := GetCheConfigMapData(deployContext)
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}
