//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package solver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/eclipse-che/che-operator/pkg/common/test"

	dwConstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	controller "github.com/eclipse-che/che-operator/controllers/devworkspace"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	constants "github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbac "k8s.io/api/rbac/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
	utilruntime.Must(dwo.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(chev2.AddToScheme(scheme))

	return scheme
}

func getSpecObjectsForManager(t *testing.T, mgr *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, additionalInitialObjects ...runtime.Object) (client.Client, solvers.RoutingSolver, solvers.RoutingObjects) {
	scheme := createTestScheme()

	allObjs := []runtime.Object{mgr, routing}
	for i := range additionalInitialObjects {
		allObjs = append(allObjs, additionalInitialObjects[i])
	}
	cl := fake.NewFakeClientWithScheme(scheme, allObjs...)

	solver, err := Getter(scheme).GetSolver(cl, "che")
	if err != nil {
		t.Fatal(err)
	}

	meta := solvers.DevWorkspaceMetadata{
		DevWorkspaceId: routing.Spec.DevWorkspaceId,
		Namespace:      routing.GetNamespace(),
		PodSelector:    routing.Spec.PodSelector,
	}

	// we need to do 1 round of che manager reconciliation so that the solver gets initialized
	cheRecon := controller.New(cl, scheme)
	_, err = cheRecon.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: mgr.Name, Namespace: mgr.Namespace}})
	if err != nil {
		t.Fatal(err)
	}

	objs, err := solver.GetSpecObjects(routing, meta)
	if err != nil {
		t.Fatal(err)
	}

	// set owner references for the routing objects
	for idx := range objs.Services {
		err := controllerutil.SetControllerReference(routing, &objs.Services[idx], scheme)
		if err != nil {
			t.Fatal(err)
		}
	}
	for idx := range objs.Ingresses {
		err := controllerutil.SetControllerReference(routing, &objs.Ingresses[idx], scheme)
		if err != nil {
			t.Fatal(err)
		}
	}
	for idx := range objs.Routes {
		err := controllerutil.SetControllerReference(routing, &objs.Routes[idx], scheme)
		if err != nil {
			t.Fatal(err)
		}
	}

	// now we need a second round of che manager reconciliation so that it proclaims the che gateway as established
	cheRecon.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "che", Namespace: "ns"}})

	return cl, solver, objs
}

func getSpecObjects(t *testing.T, routing *dwo.DevWorkspaceRouting) (client.Client, solvers.RoutingSolver, solvers.RoutingObjects) {
	return getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	}, routing, userProfileSecret("username"))
}

func subdomainDevWorkspaceRouting() *dwo.DevWorkspaceRouting {
	return &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "workspace.devfile.io/v1alpha2",
					Kind:       "DevWorkspace",
					Name:       "my-workspace",
					UID:        "uid",
				},
			},
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
			RoutingClass:   "che",
			Endpoints: map[string]dwo.EndpointList{
				"m1": {
					{
						Name:       "e1",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "https",
						Path:       "/1/",
					},
					{
						Name:       "e2",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "http",
						Path:       "/2.js",
						Secure:     true,
					},
					{
						Name:       "e3",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
					},
				},
			},
		},
	}
}

func relocatableDevWorkspaceRouting() *dwo.DevWorkspaceRouting {
	return &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "workspace.devfile.io/v1alpha2",
					Kind:       "DevWorkspace",
					Name:       "my-workspace",
					UID:        "uid",
				},
			},
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
			RoutingClass:   "che",
			Endpoints: map[string]dwo.EndpointList{
				"m1": {
					{
						Name:       "e1",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "https",
						Path:       "/1/",
						Attributes: dwo.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
							string(dwo.TypeEndpointAttribute):        apiext.JSON{Raw: []byte("\"main\"")},
						},
					},
					{
						Name:       "e2",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "http",
						Path:       "/2.js",
						Secure:     true,
						Attributes: dwo.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
					{
						Name:       "e3",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
						Attributes: dwo.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
			},
		},
	}
}

func userProfileSecret(username string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "user-profile",
			Namespace:  "ws",
			Finalizers: []string{controller.FinalizerName},
		},
		Data: map[string][]byte{
			"name": []byte(username),
		},
	}
}

func TestCreateRelocatedObjectsK8S(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	cl, _, objs := getSpecObjects(t, relocatableDevWorkspaceRouting())

	t.Run("noIngresses", func(t *testing.T) {
		if len(objs.Ingresses) != 0 {
			t.Error()
		}
	})

	t.Run("noRoutes", func(t *testing.T) {
		if len(objs.Routes) != 0 {
			t.Error()
		}
	})

	t.Run("testPodAdditions", func(t *testing.T) {
		if len(objs.PodAdditions.Containers) != 1 || objs.PodAdditions.Containers[0].Name != wsGatewayName {
			t.Error("expected Container pod addition with Workspace Gateway. Got ", objs.PodAdditions)
		}
		if len(objs.PodAdditions.Volumes) != 1 || objs.PodAdditions.Volumes[0].Name != wsGatewayName {
			t.Error("expected Volume pod addition for workspace gateway. Got ", objs.PodAdditions)
		}

		if objs.PodAdditions.Containers[0].Resources.Requests.Memory() == nil {
			t.Error("expected addition pod Container Memory request to be set")
		}
		if objs.PodAdditions.Containers[0].Resources.Requests.Cpu() == nil {
			t.Error("expected addition po Container CPU request to be set")
		}

		if objs.PodAdditions.Containers[0].Resources.Limits.Memory() == nil {
			t.Error("expected addition po Container Memory limit to be set")
		}

		if objs.PodAdditions.Containers[0].Resources.Limits.Cpu() == nil {
			t.Error("expected addition po Container CPU limit to be set")
		}
	})

	for i := range objs.Services {
		t.Run(fmt.Sprintf("service-%d", i), func(t *testing.T) {
			svc := &objs.Services[i]
			if svc.Annotations[defaults.ConfigAnnotationCheManagerName] != "che" {
				t.Errorf("The name of the associated che manager should have been recorded in the service annotation")
			}

			if svc.Annotations[defaults.ConfigAnnotationCheManagerNamespace] != "ns" {
				t.Errorf("The namespace of the associated che manager should have been recorded in the service annotation")
			}

			if svc.Labels[dwConstants.DevWorkspaceIDLabel] != "wsid" {
				t.Errorf("The workspace ID should be recorded in the service labels")
			}
		})
	}

	t.Run("traefikConfig", func(t *testing.T) {
		cms := &corev1.ConfigMapList{}
		cl.List(context.TODO(), cms)

		assert.Len(t, cms.Items, 2)

		var workspaceMainCfg *corev1.ConfigMap
		var workspaceCfg *corev1.ConfigMap
		for _, cfg := range cms.Items {
			if cfg.Name == "wsid-route" && cfg.Namespace == "ns" {
				workspaceMainCfg = cfg.DeepCopy()
			}
			if cfg.Name == "wsid-route" && cfg.Namespace == "ws" {
				workspaceCfg = cfg.DeepCopy()
			}
		}

		assert.NotNil(t, workspaceMainCfg)

		traefikMainWorkspaceConfig := workspaceMainCfg.Data["wsid.yml"]
		assert.NotEmpty(t, traefikMainWorkspaceConfig)

		traefikWorkspaceConfig := workspaceCfg.Data["workspace.yml"]
		assert.NotEmpty(t, traefikWorkspaceConfig)

		workspaceConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig))
		assert.Len(t, workspaceConfig.HTTP.Routers, 2)

		wsid := "wsid-m1-9999"
		assert.Contains(t, workspaceConfig.HTTP.Routers, wsid)
		assert.Len(t, workspaceConfig.HTTP.Routers[wsid].Middlewares, 2)
		assert.Len(t, workspaceConfig.HTTP.Middlewares, 3)

		mwares := []string{wsid + gateway.StripPrefixMiddlewareSuffix}
		for _, mware := range mwares {
			assert.Contains(t, workspaceConfig.HTTP.Middlewares, mware)
			found := false
			for _, r := range workspaceConfig.HTTP.Routers[wsid].Middlewares {
				if r == mware {
					found = true
				}
			}
			assert.True(t, found)
		}

		workspaceMainConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikMainWorkspaceConfig), &workspaceMainConfig))
		assert.Len(t, workspaceMainConfig.HTTP.Middlewares, 5)

		wsid = "wsid"
		mwares = []string{
			wsid + gateway.AuthMiddlewareSuffix,
			wsid + gateway.StripPrefixMiddlewareSuffix,
			wsid + gateway.HeadersMiddlewareSuffix,
			wsid + gateway.ErrorsMiddlewareSuffix,
			wsid + gateway.RetryMiddlewareSuffix}
		for _, mware := range mwares {
			assert.Contains(t, workspaceMainConfig.HTTP.Middlewares, mware)

			found := false
			for _, r := range workspaceMainConfig.HTTP.Routers[wsid].Middlewares {
				if r == mware {
					found = true
				}
			}
			assert.Truef(t, found, "traefik config route doesn't set middleware '%s'", mware)
		}

		t.Run("testEndpointInMainWorkspaceRoute", func(t *testing.T) {
			assert.Contains(t, workspaceMainConfig.HTTP.Routers, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[wsid].Service, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[wsid].Rule, "PathPrefix(`/username/my-workspace`)")
		})

		t.Run("testServerTransportInMainWorkspaceRoute", func(t *testing.T) {
			serverTransportName := wsid

			assert.Len(t, workspaceMainConfig.HTTP.ServersTransports, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.ServersTransports, serverTransportName)

			assert.Len(t, workspaceMainConfig.HTTP.Services, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.Services, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Services[wsid].LoadBalancer.ServersTransport, serverTransportName)
		})

		t.Run("testHealthzEndpointInMainWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-9999-healthz"
			assert.Contains(t, workspaceMainConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Service, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Rule, "Path(`/username/my-workspace/9999/healthz`)")
			assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.StripPrefixMiddlewareSuffix)
			assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.HeaderRewriteMiddlewareSuffix)
		})

		t.Run("testHealthzEndpointInWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-m1-9999-healthz"
			assert.Contains(t, workspaceConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Service, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Rule, "Path(`/9999/healthz`)")
			assert.NotContains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.StripPrefixMiddlewareSuffix)
		})

	})
}

func TestCreateRelocatedObjectsK8SLegacy(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	cl, _, objs := getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	}, relocatableDevWorkspaceRouting())

	t.Run("noIngresses", func(t *testing.T) {
		if len(objs.Ingresses) != 0 {
			t.Error()
		}
	})

	t.Run("noRoutes", func(t *testing.T) {
		if len(objs.Routes) != 0 {
			t.Error()
		}
	})

	t.Run("traefikConfig", func(t *testing.T) {
		cms := &corev1.ConfigMapList{}
		cl.List(context.TODO(), cms)

		assert.Len(t, cms.Items, 2)

		var workspaceMainCfg *corev1.ConfigMap
		var workspaceCfg *corev1.ConfigMap
		for _, cfg := range cms.Items {
			if cfg.Name == "wsid-route" && cfg.Namespace == "ns" {
				workspaceMainCfg = cfg.DeepCopy()
			}
			if cfg.Name == "wsid-route" && cfg.Namespace == "ws" {
				workspaceCfg = cfg.DeepCopy()
			}
		}

		assert.NotNil(t, workspaceMainCfg)

		traefikMainWorkspaceConfig := workspaceMainCfg.Data["wsid.yml"]
		assert.NotEmpty(t, traefikMainWorkspaceConfig)

		traefikWorkspaceConfig := workspaceCfg.Data["workspace.yml"]
		assert.NotEmpty(t, traefikWorkspaceConfig)

		workspaceConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig))
		assert.Len(t, workspaceConfig.HTTP.Routers, 2)

		wsid := "wsid-m1-9999"
		assert.Contains(t, workspaceConfig.HTTP.Routers, wsid)
		assert.Len(t, workspaceConfig.HTTP.Routers[wsid].Middlewares, 2)
		assert.Len(t, workspaceConfig.HTTP.Middlewares, 3)

		mwares := []string{wsid + gateway.StripPrefixMiddlewareSuffix}
		for _, mware := range mwares {
			assert.Contains(t, workspaceConfig.HTTP.Middlewares, mware)
			found := false
			for _, r := range workspaceConfig.HTTP.Routers[wsid].Middlewares {
				if r == mware {
					found = true
				}
			}
			assert.True(t, found)
		}

		workspaceMainConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikMainWorkspaceConfig), &workspaceMainConfig))
		assert.Len(t, workspaceMainConfig.HTTP.Middlewares, 5)

		wsid = "wsid"
		mwares = []string{
			wsid + gateway.AuthMiddlewareSuffix,
			wsid + gateway.StripPrefixMiddlewareSuffix,
			wsid + gateway.HeadersMiddlewareSuffix,
			wsid + gateway.ErrorsMiddlewareSuffix,
			wsid + gateway.RetryMiddlewareSuffix}
		for _, mware := range mwares {
			assert.Contains(t, workspaceMainConfig.HTTP.Middlewares, mware)

			found := false
			for _, r := range workspaceMainConfig.HTTP.Routers[wsid].Middlewares {
				if r == mware {
					found = true
				}
			}
			assert.Truef(t, found, "traefik config route doesn't set middleware '%s'", mware)
		}

		t.Run("testEndpointInMainWorkspaceRoute", func(t *testing.T) {
			assert.Contains(t, workspaceMainConfig.HTTP.Routers, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[wsid].Service, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[wsid].Rule, fmt.Sprintf("PathPrefix(`/%s`)", wsid))
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[wsid].Priority, 100+len("/"+wsid))
		})

		t.Run("testServerTransportInMainWorkspaceRoute", func(t *testing.T) {
			serverTransportName := wsid

			assert.Len(t, workspaceMainConfig.HTTP.ServersTransports, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.ServersTransports, serverTransportName)

			assert.Len(t, workspaceMainConfig.HTTP.Services, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.Services, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Services[wsid].LoadBalancer.ServersTransport, serverTransportName)
		})

		t.Run("testHealthzEndpointInMainWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-9999-healthz"
			assert.Contains(t, workspaceMainConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Service, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Rule, "Path(`/wsid/m1/9999/healthz`)")
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Priority, 101+len("/"+wsid))
			assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.StripPrefixMiddlewareSuffix)
			assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.HeaderRewriteMiddlewareSuffix)
		})

		t.Run("testHealthzEndpointInWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-m1-9999-healthz"
			assert.Contains(t, workspaceConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Service, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Rule, "Path(`/m1/9999/healthz`)")
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Priority, 101+len("/m1/9999"))
			assert.NotContains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.StripPrefixMiddlewareSuffix)
		})

	})
}

func TestCreateRelocatedObjectsOpenshift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	cl, _, objs := getSpecObjects(t, relocatableDevWorkspaceRouting())

	assert.Empty(t, objs.Ingresses)
	assert.Empty(t, objs.Routes)

	t.Run("testPodAdditions", func(t *testing.T) {
		assert.Len(t, objs.PodAdditions.Containers, 1)
		assert.Equal(t, objs.PodAdditions.Containers[0].Name, wsGatewayName)

		assert.Len(t, objs.PodAdditions.Volumes, 1)
		assert.Equal(t, objs.PodAdditions.Volumes[0].Name, wsGatewayName)
	})

	t.Run("traefikConfig", func(t *testing.T) {
		cms := &corev1.ConfigMapList{}
		cl.List(context.TODO(), cms)

		assert.Len(t, cms.Items, 2)

		var workspaceMainCfg *corev1.ConfigMap
		var workspaceCfg *corev1.ConfigMap
		for _, cfg := range cms.Items {
			if cfg.Name == "wsid-route" && cfg.Namespace == "ns" {
				workspaceMainCfg = cfg.DeepCopy()
			}
			if cfg.Name == "wsid-route" && cfg.Namespace == "ws" {
				workspaceCfg = cfg.DeepCopy()
			}
		}

		assert.NotNil(t, workspaceMainCfg, "traefik configuration for the workspace not found")

		traefikMainWorkspaceConfig := workspaceMainCfg.Data["wsid.yml"]
		assert.NotEmpty(t, traefikMainWorkspaceConfig, "No traefik config file found in the main workspace config configmap")

		traefikWorkspaceConfig := workspaceCfg.Data["workspace.yml"]
		assert.NotEmpty(t, traefikWorkspaceConfig, "No traefik config file found in the workspace config configmap")

		workspaceConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig))

		wsid := "wsid-m1-9999"
		assert.Contains(t, workspaceConfig.HTTP.Routers, wsid)
		assert.Len(t, workspaceConfig.HTTP.Routers[wsid].Middlewares, 2)

		workspaceMainConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikMainWorkspaceConfig), &workspaceMainConfig))
		assert.Len(t, workspaceMainConfig.HTTP.Middlewares, 6)

		wsid = "wsid"
		mwares := []string{
			wsid + gateway.AuthMiddlewareSuffix,
			wsid + gateway.StripPrefixMiddlewareSuffix,
			wsid + gateway.HeaderRewriteMiddlewareSuffix,
			wsid + gateway.HeadersMiddlewareSuffix,
			wsid + gateway.ErrorsMiddlewareSuffix,
			wsid + gateway.RetryMiddlewareSuffix}
		for _, mware := range mwares {
			assert.Contains(t, workspaceMainConfig.HTTP.Middlewares, mware)

			found := false
			for _, r := range workspaceMainConfig.HTTP.Routers[wsid].Middlewares {
				if r == mware {
					found = true
				}
			}
			assert.Truef(t, found, "traefik config route doesn't set middleware '%s'", mware)
		}

		t.Run("testServerTransportInMainWorkspaceRoute", func(t *testing.T) {
			serverTransportName := wsid

			assert.Len(t, workspaceMainConfig.HTTP.ServersTransports, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.ServersTransports, serverTransportName)

			assert.Len(t, workspaceMainConfig.HTTP.Services, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.Services, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Services[wsid].LoadBalancer.ServersTransport, serverTransportName)
		})

		t.Run("testHealthzEndpointInMainWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-9999-healthz"
			assert.Contains(t, workspaceMainConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Service, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Rule, "Path(`/username/my-workspace/9999/healthz`)")
			assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.StripPrefixMiddlewareSuffix)
			assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.HeaderRewriteMiddlewareSuffix)
		})

		t.Run("testHealthzEndpointInWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-m1-9999-healthz"
			assert.Contains(t, workspaceConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Service, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Rule, "Path(`/9999/healthz`)")
			assert.NotContains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.StripPrefixMiddlewareSuffix)
		})
	})
}

func TestCreateRelocatedObjectsOpenshiftLegacy(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	cl, _, objs := getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	}, relocatableDevWorkspaceRouting())

	assert.Empty(t, objs.Ingresses)
	assert.Empty(t, objs.Routes)

	t.Run("traefikConfig", func(t *testing.T) {
		cms := &corev1.ConfigMapList{}
		cl.List(context.TODO(), cms)

		assert.Len(t, cms.Items, 2)

		var workspaceMainCfg *corev1.ConfigMap
		var workspaceCfg *corev1.ConfigMap
		for _, cfg := range cms.Items {
			if cfg.Name == "wsid-route" && cfg.Namespace == "ns" {
				workspaceMainCfg = cfg.DeepCopy()
			}
			if cfg.Name == "wsid-route" && cfg.Namespace == "ws" {
				workspaceCfg = cfg.DeepCopy()
			}
		}

		assert.NotNil(t, workspaceMainCfg, "traefik configuration for the workspace not found")

		traefikMainWorkspaceConfig := workspaceMainCfg.Data["wsid.yml"]
		assert.NotEmpty(t, traefikMainWorkspaceConfig, "No traefik config file found in the main workspace config configmap")

		traefikWorkspaceConfig := workspaceCfg.Data["workspace.yml"]
		assert.NotEmpty(t, traefikWorkspaceConfig, "No traefik config file found in the workspace config configmap")

		workspaceConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig))

		wsid := "wsid-m1-9999"
		assert.Contains(t, workspaceConfig.HTTP.Routers, wsid)
		assert.Len(t, workspaceConfig.HTTP.Routers[wsid].Middlewares, 2)

		workspaceMainConfig := gateway.TraefikConfig{}
		assert.NoError(t, yaml.Unmarshal([]byte(traefikMainWorkspaceConfig), &workspaceMainConfig))
		assert.Len(t, workspaceMainConfig.HTTP.Middlewares, 6)

		wsid = "wsid"
		mwares := []string{
			wsid + gateway.AuthMiddlewareSuffix,
			wsid + gateway.StripPrefixMiddlewareSuffix,
			wsid + gateway.HeaderRewriteMiddlewareSuffix,
			wsid + gateway.HeadersMiddlewareSuffix,
			wsid + gateway.ErrorsMiddlewareSuffix,
			wsid + gateway.RetryMiddlewareSuffix}
		for _, mware := range mwares {
			assert.Contains(t, workspaceMainConfig.HTTP.Middlewares, mware)

			found := false
			for _, r := range workspaceMainConfig.HTTP.Routers[wsid].Middlewares {
				if r == mware {
					found = true
				}
			}
			assert.Truef(t, found, "traefik config route doesn't set middleware '%s'", mware)
		}

		t.Run("testServerTransportInMainWorkspaceRoute", func(t *testing.T) {
			serverTransportName := wsid

			assert.Len(t, workspaceMainConfig.HTTP.ServersTransports, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.ServersTransports, serverTransportName)

			assert.Len(t, workspaceMainConfig.HTTP.Services, 1)
			assert.Contains(t, workspaceMainConfig.HTTP.Services, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Services[wsid].LoadBalancer.ServersTransport, serverTransportName)
		})

		t.Run("testHealthzEndpointInMainWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-9999-healthz"
			assert.Contains(t, workspaceMainConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Service, wsid)
			assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Rule, "Path(`/wsid/m1/9999/healthz`)")
			assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.StripPrefixMiddlewareSuffix)
			assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, "wsid"+gateway.HeaderRewriteMiddlewareSuffix)
		})

		t.Run("testHealthzEndpointInWorkspaceRoute", func(t *testing.T) {
			healthzName := "wsid-m1-9999-healthz"
			assert.Contains(t, workspaceConfig.HTTP.Routers, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Service, healthzName)
			assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Rule, "Path(`/m1/9999/healthz`)")
			assert.NotContains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.AuthMiddlewareSuffix)
			assert.Contains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.StripPrefixMiddlewareSuffix)
		})
	})
}

func TestUniqueMainEndpoint(t *testing.T) {
	wsid := "wsid123"

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	cl, _, _ := getSpecObjects(t, &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "workspace.devfile.io/v1alpha2",
					Kind:       "DevWorkspace",
					Name:       "my-workspace",
					UID:        "uid",
				},
			},
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: wsid,
			RoutingClass:   "che",
			Endpoints: map[string]dwo.EndpointList{
				"m1": {
					{
						Name:       "e1",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "https",
						Path:       "/1/",
						Attributes: dwo.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
							string(dwo.TypeEndpointAttribute):        apiext.JSON{Raw: []byte("\"main\"")},
							uniqueEndpointAttributeName:              apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
			},
		},
	})

	cms := &corev1.ConfigMapList{}
	cl.List(context.TODO(), cms)

	assert.Len(t, cms.Items, 2)

	var workspaceMainCfg *corev1.ConfigMap
	var workspaceCfg *corev1.ConfigMap
	for _, cfg := range cms.Items {
		if cfg.Name == wsid+"-route" && cfg.Namespace == "ns" {
			workspaceMainCfg = cfg.DeepCopy()
		}
		if cfg.Name == wsid+"-route" && cfg.Namespace == "ws" {
			workspaceCfg = cfg.DeepCopy()
		}
	}

	traefikWorkspaceConfig := workspaceCfg.Data["workspace.yml"]
	workspaceConfig := gateway.TraefikConfig{}
	assert.NoError(t, yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig))

	traefikMainWorkspaceConfig := workspaceMainCfg.Data[wsid+".yml"]
	workspaceMainConfig := gateway.TraefikConfig{}
	assert.NoError(t, yaml.Unmarshal([]byte(traefikMainWorkspaceConfig), &workspaceMainConfig))

	t.Run("testHealthzEndpointInMainWorkspaceRoute", func(t *testing.T) {
		healthzName := wsid + "-e1-healthz"
		assert.Contains(t, workspaceMainConfig.HTTP.Routers, healthzName)
		assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Service, wsid)
		assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Rule, "Path(`/username/my-workspace/e1/healthz`)")
		assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, wsid+gateway.AuthMiddlewareSuffix)
		assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, wsid+gateway.StripPrefixMiddlewareSuffix)
		assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, wsid+gateway.HeaderRewriteMiddlewareSuffix)
	})

	t.Run("testHealthzEndpointInWorkspaceRoute", func(t *testing.T) {
		healthzName := wsid + "-m1-e1-healthz"
		assert.Contains(t, workspaceConfig.HTTP.Routers, healthzName)
		assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Service, healthzName)
		assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Rule, "Path(`/e1/healthz`)")
		assert.NotContains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.AuthMiddlewareSuffix)
		assert.Contains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.StripPrefixMiddlewareSuffix)
	})
}

func TestUniqueMainEndpointLegacy(t *testing.T) {
	wsid := "wsid123"

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	routing := &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "workspace.devfile.io/v1alpha2",
					Kind:       "DevWorkspace",
					Name:       "my-workspace",
					UID:        "uid",
				},
			},
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: wsid,
			RoutingClass:   "che",
			Endpoints: map[string]dwo.EndpointList{
				"m1": {
					{
						Name:       "e1",
						TargetPort: 9999,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "https",
						Path:       "/1/",
						Attributes: dwo.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
							string(dwo.TypeEndpointAttribute):        apiext.JSON{Raw: []byte("\"main\"")},
							uniqueEndpointAttributeName:              apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
			},
		},
	}

	cl, _, _ := getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	}, routing)

	cms := &corev1.ConfigMapList{}
	cl.List(context.TODO(), cms)

	assert.Len(t, cms.Items, 2)

	var workspaceMainCfg *corev1.ConfigMap
	var workspaceCfg *corev1.ConfigMap
	for _, cfg := range cms.Items {
		if cfg.Name == wsid+"-route" && cfg.Namespace == "ns" {
			workspaceMainCfg = cfg.DeepCopy()
		}
		if cfg.Name == wsid+"-route" && cfg.Namespace == "ws" {
			workspaceCfg = cfg.DeepCopy()
		}
	}

	traefikWorkspaceConfig := workspaceCfg.Data["workspace.yml"]
	workspaceConfig := gateway.TraefikConfig{}
	assert.NoError(t, yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig))

	traefikMainWorkspaceConfig := workspaceMainCfg.Data[wsid+".yml"]
	workspaceMainConfig := gateway.TraefikConfig{}
	assert.NoError(t, yaml.Unmarshal([]byte(traefikMainWorkspaceConfig), &workspaceMainConfig))

	t.Run("testHealthzEndpointInMainWorkspaceRoute", func(t *testing.T) {
		healthzName := wsid + "-e1-healthz"
		assert.Contains(t, workspaceMainConfig.HTTP.Routers, healthzName)
		assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Service, wsid)
		assert.Equal(t, workspaceMainConfig.HTTP.Routers[healthzName].Rule, "Path(`/"+wsid+"/m1/e1/healthz`)")
		assert.NotContains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, wsid+gateway.AuthMiddlewareSuffix)
		assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, wsid+gateway.StripPrefixMiddlewareSuffix)
		assert.Contains(t, workspaceMainConfig.HTTP.Routers[healthzName].Middlewares, wsid+gateway.HeaderRewriteMiddlewareSuffix)
	})

	t.Run("testHealthzEndpointInWorkspaceRoute", func(t *testing.T) {
		healthzName := wsid + "-m1-e1-healthz"
		assert.Contains(t, workspaceConfig.HTTP.Routers, healthzName)
		assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Service, healthzName)
		assert.Equal(t, workspaceConfig.HTTP.Routers[healthzName].Rule, "Path(`/m1/e1/healthz`)")
		assert.NotContains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.AuthMiddlewareSuffix)
		assert.Contains(t, workspaceConfig.HTTP.Routers[healthzName].Middlewares, healthzName+gateway.StripPrefixMiddlewareSuffix)
	})
}

func TestCreateSubDomainObjects(t *testing.T) {
	testCommon := func(infra infrastructure.Type) solvers.RoutingObjects {
		infrastructure.InitializeForTesting(infra)

		cl, _, objs := getSpecObjects(t, subdomainDevWorkspaceRouting())

		t.Run("testPodAdditions", func(t *testing.T) {
			if len(objs.PodAdditions.Containers) != 1 || objs.PodAdditions.Containers[0].Name != wsGatewayName {
				t.Error("expected Container pod addition with Workspace Gateway. Got ", objs.PodAdditions)
			}
			if len(objs.PodAdditions.Volumes) != 1 || objs.PodAdditions.Volumes[0].Name != wsGatewayName {
				t.Error("expected Volume pod addition for workspace gateway. Got ", objs.PodAdditions)
			}
		})

		for i := range objs.Services {
			t.Run(fmt.Sprintf("service-%d", i), func(t *testing.T) {
				svc := &objs.Services[i]
				if svc.Annotations[defaults.ConfigAnnotationCheManagerName] != "che" {
					t.Errorf("The name of the associated che manager should have been recorded in the service annotation")
				}

				if svc.Annotations[defaults.ConfigAnnotationCheManagerNamespace] != "ns" {
					t.Errorf("The namespace of the associated che manager should have been recorded in the service annotation")
				}

				if svc.Labels[dwConstants.DevWorkspaceIDLabel] != "wsid" {
					t.Errorf("The workspace ID should be recorded in the service labels")
				}
			})
		}

		t.Run("noWorkspaceTraefikConfig", func(t *testing.T) {
			cms := &corev1.ConfigMapList{}
			cl.List(context.TODO(), cms)

			if len(cms.Items) != 2 {
				t.Errorf("there should be 2 configmaps create but found: %d", len(cms.Items))
			}
		})

		return objs
	}

	t.Run("expectedIngresses", func(t *testing.T) {
		objs := testCommon(infrastructure.Kubernetes)
		if len(objs.Ingresses) != 3 {
			t.Error("Expected 3 ingress, found ", len(objs.Ingresses))
		}
		if objs.Ingresses[0].Spec.Rules[0].Host != "username-my-workspace-e1.down.on.earth" {
			t.Error("Expected Ingress host 'username-my-workspace-e1.down.on.earth', but got ", objs.Ingresses[0].Spec.Rules[0].Host)
		}
		if objs.Ingresses[1].Spec.Rules[0].Host != "username-my-workspace-e2.down.on.earth" {
			t.Error("Expected Ingress host 'username-my-workspace-e2.down.on.earth', but got ", objs.Ingresses[1].Spec.Rules[0].Host)
		}
		if objs.Ingresses[2].Spec.Rules[0].Host != "username-my-workspace-e3.down.on.earth" {
			t.Error("Expected Ingress host 'username-my-workspace-e3.down.on.earth', but got ", objs.Ingresses[2].Spec.Rules[0].Host)
		}
	})

	t.Run("expectedRoutes", func(t *testing.T) {
		objs := testCommon(infrastructure.OpenShiftv4)
		if len(objs.Routes) != 3 {
			t.Error("Expected 3 Routes, found ", len(objs.Routes))
		}
		if objs.Routes[0].Spec.Host != "username-my-workspace-e1.down.on.earth" {
			t.Error("Expected Route host 'username-my-workspace-e1.down.on.earth', but got ", objs.Routes[0].Spec.Host)
		}
	})
}

func TestCreateSubDomainObjectsLegacy(t *testing.T) {
	testCommon := func(infra infrastructure.Type) solvers.RoutingObjects {
		infrastructure.InitializeForTesting(infra)

		cl, _, objs := getSpecObjectsForManager(t, &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "che",
				Namespace:  "ns",
				Finalizers: []string{controller.FinalizerName},
			},
			Spec: chev2.CheClusterSpec{
				Networking: chev2.CheClusterSpecNetworking{
					Domain:   "down.on.earth",
					Hostname: "over.the.rainbow",
				},
			},
		}, subdomainDevWorkspaceRouting())

		t.Run("testPodAdditions", func(t *testing.T) {
			if len(objs.PodAdditions.Containers) != 1 || objs.PodAdditions.Containers[0].Name != wsGatewayName {
				t.Error("expected Container pod addition with Workspace Gateway. Got ", objs.PodAdditions)
			}
			if len(objs.PodAdditions.Volumes) != 1 || objs.PodAdditions.Volumes[0].Name != wsGatewayName {
				t.Error("expected Volume pod addition for workspace gateway. Got ", objs.PodAdditions)
			}
		})

		for i := range objs.Services {
			t.Run(fmt.Sprintf("service-%d", i), func(t *testing.T) {
				svc := &objs.Services[i]
				if svc.Annotations[defaults.ConfigAnnotationCheManagerName] != "che" {
					t.Errorf("The name of the associated che manager should have been recorded in the service annotation")
				}

				if svc.Annotations[defaults.ConfigAnnotationCheManagerNamespace] != "ns" {
					t.Errorf("The namespace of the associated che manager should have been recorded in the service annotation")
				}

				if svc.Labels[dwConstants.DevWorkspaceIDLabel] != "wsid" {
					t.Errorf("The workspace ID should be recorded in the service labels")
				}
			})
		}

		t.Run("noWorkspaceTraefikConfig", func(t *testing.T) {
			cms := &corev1.ConfigMapList{}
			cl.List(context.TODO(), cms)

			if len(cms.Items) != 2 {
				t.Errorf("there should be 2 configmaps create but found: %d", len(cms.Items))
			}
		})

		return objs
	}

	t.Run("expectedIngresses", func(t *testing.T) {
		objs := testCommon(infrastructure.Kubernetes)
		if len(objs.Ingresses) != 3 {
			t.Error("Expected 3 ingress, found ", len(objs.Ingresses))
		}
		if objs.Ingresses[0].Spec.Rules[0].Host != "wsid-1.down.on.earth" {
			t.Error("Expected Ingress host 'wsid-1.down.on.earth', but got ", objs.Ingresses[0].Spec.Rules[0].Host)
		}
		if objs.Ingresses[1].Spec.Rules[0].Host != "wsid-2.down.on.earth" {
			t.Error("Expected Ingress host 'wsid-2.down.on.earth', but got ", objs.Ingresses[1].Spec.Rules[0].Host)
		}
		if objs.Ingresses[2].Spec.Rules[0].Host != "wsid-3.down.on.earth" {
			t.Error("Expected Ingress host 'wsid-3.down.on.earth', but got ", objs.Ingresses[2].Spec.Rules[0].Host)
		}
	})

	t.Run("expectedRoutes", func(t *testing.T) {
		objs := testCommon(infrastructure.OpenShiftv4)
		if len(objs.Routes) != 3 {
			t.Error("Expected 3 Routes, found ", len(objs.Routes))
		}
		if objs.Routes[0].Spec.Host != "wsid-1.down.on.earth" {
			t.Error("Expected Route host 'wsid-1.down.on.earth', but got ", objs.Routes[0].Spec.Host)
		}
	})
}

func TestReportRelocatableExposedEndpoints(t *testing.T) {
	// kubernetes
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	routing := relocatableDevWorkspaceRouting()
	_, solver, objs := getSpecObjects(t, routing)

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 component.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://over.the.rainbow/username/my-workspace/9999/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/username/my-workspace/9999/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://over.the.rainbow/username/my-workspace/9999/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/username/my-workspace/9999/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "https://over.the.rainbow/username/my-workspace/9999/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/username/my-workspace/9999/", e3.Url)
	}
}

func TestReportRelocatableExposedEndpointsLegacy(t *testing.T) {
	// kubernetes
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	routing := relocatableDevWorkspaceRouting()
	_, solver, objs := getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	}, routing)

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 component.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://over.the.rainbow/wsid/m1/9999/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://over.the.rainbow/wsid/m1/9999/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "https://over.the.rainbow/wsid/m1/9999/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/", e3.Url)
	}
}

func TestExposeEndpoints(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	routing := &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "workspace.devfile.io/v1alpha2",
					Kind:       "DevWorkspace",
					Name:       "my-workspace",
					UID:        "uid",
				},
			},
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
			RoutingClass:   "che",
			Endpoints: map[string]dwo.EndpointList{
				"server-internal": {
					{
						Name:       "server-int",
						TargetPort: 8081,
						Exposure:   dwo.InternalEndpointExposure,
						Protocol:   "http",
						Attributes: map[string]apiext.JSON{
							"urlRewriteSupported": apiext.JSON{Raw: []byte("\"true\"")},
							"cookiesAuthEnabled":  apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
				"server-internal-no-rewrite": {
					{
						Name:       "server-int-nr",
						TargetPort: 8084,
						Exposure:   dwo.InternalEndpointExposure,
						Protocol:   "http",
					},
				},
				"server-none": {
					{
						Name:       "server-int",
						TargetPort: 8080,
						Exposure:   dwo.NoneEndpointExposure,
						Protocol:   "http",
						Attributes: map[string]apiext.JSON{
							"urlRewriteSupported": apiext.JSON{Raw: []byte("\"true\"")},
							"cookiesAuthEnabled":  apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
				"server-none-no-rewrite": {
					{
						Name:       "server-none-nr",
						TargetPort: 8083,
						Exposure:   dwo.NoneEndpointExposure,
						Protocol:   "http",
					},
				},
				"server-public": {
					{
						Name:       "server-pub",
						TargetPort: 8082,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "http",
						Attributes: map[string]apiext.JSON{
							"urlRewriteSupported": apiext.JSON{Raw: []byte("\"true\"")},
							"cookiesAuthEnabled":  apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
				"server-public-no-rewrite": {
					{
						Name:       "server-pub-nr",
						TargetPort: 8085,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "http",
					},
				},
			},
		},
	}

	_, solver, objs := getSpecObjects(t, routing)

	assert.Equal(t, 1, len(objs.Ingresses))

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	assert.Nil(t, err)
	assert.True(t, ready)
	assert.Equal(t, 4, len(exposed))

	si, ok := exposed["server-internal"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(si))
	assert.Equal(t, "server-int", si[0].Name)
	assert.Equal(t, "http://wsid-service.ws.svc:8081", si[0].Url)

	sinr, ok := exposed["server-internal-no-rewrite"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(sinr))
	assert.Equal(t, "server-int-nr", sinr[0].Name)
	assert.Equal(t, "http://wsid-service.ws.svc:8084", sinr[0].Url)

	sp, ok := exposed["server-public"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(sp))
	assert.Equal(t, "server-pub", sp[0].Name)
	assert.Equal(t, "https://over.the.rainbow/username/my-workspace/8082/", sp[0].Url)

	spnr, ok := exposed["server-public-no-rewrite"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(spnr))
	assert.Equal(t, "server-pub-nr", spnr[0].Name)
	assert.Equal(t, "http://username-my-workspace-server-pub-nr.down.on.earth/", spnr[0].Url)
}

func TestExposeEndpointsLegacy(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	routing := &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "workspace.devfile.io/v1alpha2",
					Kind:       "DevWorkspace",
					Name:       "my-workspace",
					UID:        "uid",
				},
			},
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
			RoutingClass:   "che",
			Endpoints: map[string]dwo.EndpointList{
				"server-internal": {
					{
						Name:       "server-int",
						TargetPort: 8081,
						Exposure:   dwo.InternalEndpointExposure,
						Protocol:   "http",
						Attributes: map[string]apiext.JSON{
							"urlRewriteSupported": apiext.JSON{Raw: []byte("\"true\"")},
							"cookiesAuthEnabled":  apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
				"server-internal-no-rewrite": {
					{
						Name:       "server-int-nr",
						TargetPort: 8084,
						Exposure:   dwo.InternalEndpointExposure,
						Protocol:   "http",
					},
				},
				"server-none": {
					{
						Name:       "server-int",
						TargetPort: 8080,
						Exposure:   dwo.NoneEndpointExposure,
						Protocol:   "http",
						Attributes: map[string]apiext.JSON{
							"urlRewriteSupported": apiext.JSON{Raw: []byte("\"true\"")},
							"cookiesAuthEnabled":  apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
				"server-none-no-rewrite": {
					{
						Name:       "server-none-nr",
						TargetPort: 8083,
						Exposure:   dwo.NoneEndpointExposure,
						Protocol:   "http",
					},
				},
				"server-public": {
					{
						Name:       "server-pub",
						TargetPort: 8082,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "http",
						Attributes: map[string]apiext.JSON{
							"urlRewriteSupported": apiext.JSON{Raw: []byte("\"true\"")},
							"cookiesAuthEnabled":  apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
				"server-public-no-rewrite": {
					{
						Name:       "server-pub-nr",
						TargetPort: 8085,
						Exposure:   dwo.PublicEndpointExposure,
						Protocol:   "http",
					},
				},
			},
		},
	}

	_, solver, objs := getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	}, routing)

	assert.Equal(t, 1, len(objs.Ingresses))

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	assert.Nil(t, err)
	assert.True(t, ready)
	assert.Equal(t, 4, len(exposed))

	si, ok := exposed["server-internal"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(si))
	assert.Equal(t, "server-int", si[0].Name)
	assert.Equal(t, "http://wsid-service.ws.svc:8081", si[0].Url)

	sinr, ok := exposed["server-internal-no-rewrite"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(sinr))
	assert.Equal(t, "server-int-nr", sinr[0].Name)
	assert.Equal(t, "http://wsid-service.ws.svc:8084", sinr[0].Url)

	sp, ok := exposed["server-public"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(sp))
	assert.Equal(t, "server-pub", sp[0].Name)
	assert.Equal(t, "https://over.the.rainbow/wsid/server-public/8082/", sp[0].Url)

	spnr, ok := exposed["server-public-no-rewrite"]
	assert.True(t, ok)
	assert.Equal(t, 1, len(spnr))
	assert.Equal(t, "server-pub-nr", spnr[0].Name)
	assert.Equal(t, "http://wsid-1.down.on.earth/", spnr[0].Url)
}

func TestReportSubdomainExposedEndpoints(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := subdomainDevWorkspaceRouting()
	_, solver, objs := getSpecObjects(t, routing)

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 component.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://username-my-workspace-e1.down.on.earth/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://username-my-workspace-e1.down.on.earth/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://username-my-workspace-e2.down.on.earth/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://username-my-workspace-e2.down.on.earth/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "http://username-my-workspace-e3.down.on.earth/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "http://username-my-workspace-e3.down.on.earth/", e3.Url)
	}
}

func TestReportSubdomainExposedEndpointsLongUsername(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := subdomainDevWorkspaceRouting()
	_, solver, objs := getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	},
		routing,
		// use legacy routing
		userProfileSecret("a-very-very-very-very-very-very-very-very-very-very-long-username"))

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 component.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://wsid-1.down.on.earth/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-1.down.on.earth/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://wsid-2.down.on.earth/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-2.down.on.earth/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "http://wsid-3.down.on.earth/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-3.down.on.earth/", e3.Url)
	}
}

func TestReportSubdomainExposedEndpointsLegacy(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := subdomainDevWorkspaceRouting()
	_, solver, objs := getSpecObjectsForManager(t, &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Domain:   "down.on.earth",
				Hostname: "over.the.rainbow",
			},
		},
	}, routing)

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 component.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://wsid-1.down.on.earth/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-1.down.on.earth/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://wsid-2.down.on.earth/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-2.down.on.earth/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "http://wsid-3.down.on.earth/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-3.down.on.earth/", e3.Url)
	}
}

func TestFinalize(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := relocatableDevWorkspaceRouting()
	cl, slv, _ := getSpecObjects(t, routing)

	// the create test checks that during the above call, the solver created the 2 traefik configmaps
	// (1 for the main config and the second for the devworkspace)

	// now, let the solver finalize the routing
	if err := slv.Finalize(routing); err != nil {
		t.Fatal(err)
	}

	cms := &corev1.ConfigMapList{}
	cl.List(context.TODO(), cms)

	if len(cms.Items) != 0 {
		t.Fatalf("There should be just 0 configmaps after routing finalization, but there were %d found", len(cms.Items))
	}
}

func TestEndpointsAlwaysOnSecureProtocolsWhenExposedThroughGateway(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := relocatableDevWorkspaceRouting()
	_, slv, objs := getSpecObjects(t, routing)

	exposed, ready, err := slv.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should be considered ready.")
	}

	for _, endpoints := range exposed {
		for _, endpoint := range endpoints {
			if !strings.HasPrefix(endpoint.Url, "https://") {
				t.Errorf("The endpoint %s should be exposed on https.", endpoint.Url)
			}
		}
	}
}

func TestUsesIngressAnnotationsForWorkspaceEndpointIngresses(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	mgr := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "over.the.rainbow",
				Domain:   "down.on.earth",
				Annotations: map[string]string{
					"a": "b",
				},
			},
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting(), userProfileSecret("username"))

	if len(objs.Ingresses) != 3 {
		t.Fatalf("Unexpected number of generated ingresses: %d", len(objs.Ingresses))
	}

	ingress := objs.Ingresses[0]
	if len(ingress.Annotations) != 3 {
		// 3 annotations - a => b, endpoint-name and component-name
		t.Fatalf("Unexpected number of annotations on the generated ingress: %d", len(ingress.Annotations))
	}

	if ingress.Annotations["a"] != "b" {
		t.Errorf("Unexpected value of the custom endpoint ingress annotation")
	}
}

func TestUsesCustomCertificateForWorkspaceEndpointIngresses(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	mgr := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				TlsSecretName: "tlsSecret",
				Hostname:      "beyond.comprehension",
				Domain:        "almost.trivial",
			},
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting(), userProfileSecret("username"), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tlsSecret",
			Namespace: "ns",
		},
		Data: map[string][]byte{
			"tls.key": []byte("asdf"),
			"tls.crt": []byte("qwer"),
		},
	})

	if len(objs.Ingresses) != 3 {
		t.Fatalf("Unexpected number of generated ingresses: %d", len(objs.Ingresses))
	}

	ingress := objs.Ingresses[0]

	if len(ingress.Spec.TLS) != 1 {
		t.Fatalf("Unexpected number of TLS records on the ingress: %d", len(ingress.Spec.TLS))
	}

	if ingress.Spec.TLS[0].SecretName != "wsid-endpoints" {
		t.Errorf("Unexpected name of the TLS secret on the ingress: %s", ingress.Spec.TLS[0].SecretName)
	}

	if len(ingress.Spec.TLS[0].Hosts) != 1 {
		t.Fatalf("Unexpected number of host records on the TLS spec: %d", len(ingress.Spec.TLS[0].Hosts))
	}

	if ingress.Spec.TLS[0].Hosts[0] != "username-my-workspace-e1.almost.trivial" {
		t.Errorf("Unexpected host name of the TLS spec: %s", ingress.Spec.TLS[0].Hosts[0])
	}

	ingress = objs.Ingresses[1]

	if len(ingress.Spec.TLS) != 1 {
		t.Fatalf("Unexpected number of TLS records on the ingress: %d", len(ingress.Spec.TLS))
	}

	if ingress.Spec.TLS[0].SecretName != "wsid-endpoints" {
		t.Errorf("Unexpected name of the TLS secret on the ingress: %s", ingress.Spec.TLS[0].SecretName)
	}

	if len(ingress.Spec.TLS[0].Hosts) != 1 {
		t.Fatalf("Unexpected number of host records on the TLS spec: %d", len(ingress.Spec.TLS[0].Hosts))
	}

	if ingress.Spec.TLS[0].Hosts[0] != "username-my-workspace-e2.almost.trivial" {
		t.Errorf("Unexpected host name of the TLS spec: %s", ingress.Spec.TLS[0].Hosts[0])
	}

	ingress = objs.Ingresses[2]

	if len(ingress.Spec.TLS) != 0 {
		t.Fatalf("Unexpected number of TLS records on the ingress: %d", len(ingress.Spec.TLS))
	}
}

func TestUsesCustomCertificateForWorkspaceEndpointRoutes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	mgr := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname:      "beyond.comprehension",
				TlsSecretName: "tlsSecret",
				Domain:        "almost.trivial",
			},
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting(), userProfileSecret("username"), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tlsSecret",
			Namespace: "ns",
		},
		Data: map[string][]byte{
			"tls.key": []byte("asdf"),
			"tls.crt": []byte("qwer"),
		},
	})

	if len(objs.Routes) != 3 {
		t.Fatalf("Unexpected number of generated routes: %d", len(objs.Routes))
	}

	route := objs.Routes[0]

	if route.Spec.TLS.Certificate != "qwer" {
		t.Errorf("Unexpected name of the TLS certificate on the route: %s", route.Spec.TLS.Certificate)
	}

	if route.Spec.TLS.Key != "asdf" {
		t.Errorf("Unexpected key of TLS spec: %s", route.Spec.TLS.Key)
	}

	route = objs.Routes[1]

	if route.Spec.TLS.Certificate != "qwer" {
		t.Errorf("Unexpected name of the TLS certificate on the route: %s", route.Spec.TLS.Certificate)
	}

	if route.Spec.TLS.Key != "asdf" {
		t.Errorf("Unexpected key of TLS spec: %s", route.Spec.TLS.Key)
	}

	route = objs.Routes[2]

	if route.Spec.TLS != nil {
		t.Errorf("Unexpected TLS on the route: %s", route.Spec.TLS)
	}
}

func TestOverrideGatewayContainerProvisioning(t *testing.T) {
	overrideMemoryRequest := resource.MustParse("128Mi")
	overrideCpuRequest := resource.MustParse("1")
	overrideMemoryLimit := resource.MustParse("228Mi")
	overrideCpuLimit := resource.MustParse("2")

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "test.hostname",
				Domain:   "test.domain",
			},
			DevEnvironments: chev2.CheClusterDevEnvironments{
				GatewayContainer: &chev2.Container{
					Resources: &chev2.ResourceRequirements{
						Requests: &chev2.ResourceList{
							Memory: &overrideMemoryRequest,
							Cpu:    &overrideCpuRequest,
						},
						Limits: &chev2.ResourceList{
							Memory: &overrideMemoryLimit,
							Cpu:    &overrideCpuLimit,
						},
					},
				},
			},
		},
	}

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})

	cheSolver := &CheRoutingSolver{client: deployContext.ClusterAPI.Client, scheme: deployContext.ClusterAPI.Scheme}
	objs := &solvers.RoutingObjects{}

	routing := &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
		},
	}

	err := cheSolver.provisionPodAdditions(objs, cheCluster, routing)
	assert.NoError(t, err)

	assert.Equal(t, overrideCpuRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Cpu().String())
	assert.Equal(t, overrideMemoryRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Memory().String())
	assert.Equal(t, overrideCpuLimit.String(), objs.PodAdditions.Containers[0].Resources.Limits.Cpu().String())
	assert.Equal(t, overrideMemoryLimit.String(), objs.PodAdditions.Containers[0].Resources.Limits.Memory().String())
}

func TestOverridePartialLimitsGatewayContainerProvisioning(t *testing.T) {
	overrideMemoryRequest := resource.MustParse("0")
	overrideCpuRequest := resource.MustParse("0")
	defaultMemoryLimit := resource.MustParse(constants.DefaultGatewayMemoryLimit)

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "test.hostname",
				Domain:   "test.domain",
			},
			DevEnvironments: chev2.CheClusterDevEnvironments{
				GatewayContainer: &chev2.Container{
					Resources: &chev2.ResourceRequirements{
						Requests: &chev2.ResourceList{
							Memory: &overrideMemoryRequest,
							Cpu:    &overrideCpuRequest,
						},
					},
				},
			},
		},
	}

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})

	cheSolver := &CheRoutingSolver{client: deployContext.ClusterAPI.Client, scheme: deployContext.ClusterAPI.Scheme}
	objs := &solvers.RoutingObjects{}

	routing := &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
		},
	}

	err := cheSolver.provisionPodAdditions(objs, cheCluster, routing)
	assert.NoError(t, err)

	assert.Equal(t, overrideCpuRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Cpu().String())
	assert.Equal(t, overrideMemoryRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Memory().String())
	assert.Empty(t, objs.PodAdditions.Containers[0].Resources.Limits[corev1.ResourceCPU])
	assert.Equal(t, defaultMemoryLimit.String(), objs.PodAdditions.Containers[0].Resources.Limits.Memory().String())
}

func TestOverrideGatewayEmptyContainerProvisioning(t *testing.T) {
	overrideMemoryRequest := resource.MustParse("0")
	overrideCpuRequest := resource.MustParse("0")
	overrideMemoryLimit := resource.MustParse("0")
	overrideCpuLimit := resource.MustParse("0")

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "test.hostname",
				Domain:   "test.domain",
			},
			DevEnvironments: chev2.CheClusterDevEnvironments{
				GatewayContainer: &chev2.Container{
					Resources: &chev2.ResourceRequirements{
						Requests: &chev2.ResourceList{
							Memory: &overrideMemoryRequest,
							Cpu:    &overrideCpuRequest,
						},
						Limits: &chev2.ResourceList{
							Memory: &overrideMemoryLimit,
							Cpu:    &overrideCpuLimit,
						},
					},
				},
			},
		},
	}

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})

	cheSolver := &CheRoutingSolver{client: deployContext.ClusterAPI.Client, scheme: deployContext.ClusterAPI.Scheme}
	objs := &solvers.RoutingObjects{}

	routing := &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
		},
	}

	err := cheSolver.provisionPodAdditions(objs, cheCluster, routing)
	assert.NoError(t, err)

	assert.Equal(t, overrideCpuRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Cpu().String())
	assert.Equal(t, overrideMemoryRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Memory().String())
	assert.Equal(t, overrideCpuLimit.String(), objs.PodAdditions.Containers[0].Resources.Limits.Cpu().String())
	assert.Equal(t, overrideMemoryLimit.String(), objs.PodAdditions.Containers[0].Resources.Limits.Memory().String())
}

func TestDefaultGatewayContainerProvisioning(t *testing.T) {
	defaultMemoryRequest := resource.MustParse(constants.DefaultGatewayMemoryRequest)
	defaultCpuRequest := resource.MustParse(constants.DefaultGatewayCpuRequest)
	defaultMemoryLimit := resource.MustParse(constants.DefaultGatewayMemoryLimit)

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "test.hostname",
				Domain:   "test.domain",
			},
		},
	}

	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})

	cheSolver := &CheRoutingSolver{client: deployContext.ClusterAPI.Client, scheme: deployContext.ClusterAPI.Scheme}
	objs := &solvers.RoutingObjects{}

	routing := &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
		},
	}

	err := cheSolver.provisionPodAdditions(objs, cheCluster, routing)
	assert.NoError(t, err)

	assert.Equal(t, defaultCpuRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Cpu().String())
	assert.Equal(t, defaultMemoryRequest.String(), objs.PodAdditions.Containers[0].Resources.Requests.Memory().String())
	assert.Empty(t, objs.PodAdditions.Containers[0].Resources.Limits[corev1.ResourceCPU])
	assert.Equal(t, defaultMemoryLimit.String(), objs.PodAdditions.Containers[0].Resources.Limits.Memory().String())
}
