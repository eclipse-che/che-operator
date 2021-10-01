//
// Copyright (c) 2021 Red Hat, Inc.
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
	"testing"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestGetBackupServerConfigurationNameForBackupBeforeUpdate(t *testing.T) {
	type testCase struct {
		name                string
		cheCluster          *chev1.CheCluster
		backupServerConfigs *chev1.CheBackupServerConfigurationList
		initObjects         []runtime.Object
		expectedResult      string
	}

	testCases := []testCase{
		{
			name: "Should return default backup configuration if no backup server configurations exists",
			cheCluster: &chev1.CheCluster{
				Spec: chev1.CheClusterSpec{},
			},
			backupServerConfigs: &chev1.CheBackupServerConfigurationList{
				Items: []chev1.CheBackupServerConfiguration{},
			},
			initObjects:    []runtime.Object{},
			expectedResult: "",
		},
		{
			name: "Should return only one existing configuration",
			cheCluster: &chev1.CheCluster{
				Spec: chev1.CheClusterSpec{},
			},
			backupServerConfigs: &chev1.CheBackupServerConfigurationList{
				Items: []chev1.CheBackupServerConfiguration{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "onlyExistingConfiguration",
							Namespace: "eclipse-che",
						},
					},
				},
			},
			initObjects:    []runtime.Object{},
			expectedResult: "onlyExistingConfiguration",
		},
		{
			name: "Should pick annotated backup server configuration",
			cheCluster: &chev1.CheCluster{
				Spec: chev1.CheClusterSpec{},
			},
			backupServerConfigs: &chev1.CheBackupServerConfigurationList{
				Items: []chev1.CheBackupServerConfiguration{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "noAnnotations",
							Namespace: "eclipse-che",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "otherAnnotations",
							Namespace: "eclipse-che",
							Annotations: map[string]string{
								"foo": "false", "bar": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "annotatedBackupServerConfigurationName",
							Namespace: "eclipse-che",
							Annotations: map[string]string{
								"foo": "false", DefaultBackupServerConfigLabelKey: "true", "bar": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "emptyAnnotations",
							Namespace:   "eclipse-che",
							Annotations: map[string]string{},
						},
					},
				},
			},
			initObjects:    []runtime.Object{},
			expectedResult: "annotatedBackupServerConfigurationName",
		},
		{
			name: "Should not pick annotated with false value backup server configuration",
			cheCluster: &chev1.CheCluster{
				Spec: chev1.CheClusterSpec{},
			},
			backupServerConfigs: &chev1.CheBackupServerConfigurationList{
				Items: []chev1.CheBackupServerConfiguration{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "annotatedBackupServerConfigurationName",
							Namespace: "eclipse-che",
							Annotations: map[string]string{
								"foo": "false", DefaultBackupServerConfigLabelKey: "false", "bar": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "otherAnnotations",
							Namespace: "eclipse-che",
							Annotations: map[string]string{
								"foo": "1",
							},
						},
					},
				},
			},
			initObjects:    []runtime.Object{},
			expectedResult: "",
		},
		{
			name: "Should return default backup configuration if more than one configuration exists but no annotated",
			cheCluster: &chev1.CheCluster{
				Spec: chev1.CheClusterSpec{},
			},
			backupServerConfigs: &chev1.CheBackupServerConfigurationList{
				Items: []chev1.CheBackupServerConfiguration{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "otherAnnotations",
							Namespace: "eclipse-che",
							Annotations: map[string]string{
								"foo": "false", "bar": "true",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "noAnnotations",
							Namespace: "eclipse-che",
						},
					},
				},
			},
			initObjects:    []runtime.Object{},
			expectedResult: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			chev1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.backupServerConfigs, testCase.cheCluster)

			scheme := scheme.Scheme
			chev1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.Proxy{})

			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, _ := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client:          cli,
					NonCachedClient: cli,
					Scheme:          scheme,
				},
			}

			actualResult, err := getBackupServerConfigurationNameForBackupBeforeUpdate(deployContext)
			if err != nil {
				t.Fatalf("Error getting backup server configuration name: %v", err)
			}

			if testCase.expectedResult != actualResult {
				t.Errorf("Expected backup server configuration name and returned one differ (-want, +got): %v", cmp.Diff(testCase.expectedResult, actualResult))
			}
		})
	}
}
