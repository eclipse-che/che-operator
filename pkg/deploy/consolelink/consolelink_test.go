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

package consolelink

import (
	"context"
	"fmt"
	"strings"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileConsoleLink(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	consolelink := NewConsoleLinkReconciler()
	test.EnsureReconcile(t, ctx, consolelink.Reconcile)

	consoleLink := &consolev1.ConsoleLink{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: defaults.GetConsoleLinkName()}, consoleLink)
	assert.Nil(t, err)
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
	assert.Equal(t, "https://che-host", consoleLink.Spec.Href)

	// Initialize DeletionTimestamp => checluster is being deleted
	done := consolelink.Finalize(ctx)
	assert.True(t, done)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: defaults.GetConsoleLinkName()}, &consolev1.ConsoleLink{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
}

func TestReconcileConsoleLinkWhenCheURLChanged(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://test-host",
		},
	}

	existedConsoleLink := &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.GetConsoleLinkName(),
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: "https://che-host",
				Text: defaults.GetConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  defaults.GetConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", "che-host", defaults.GetConsoleLinkImage()),
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(existedConsoleLink).Build()

	consoleLinkReconciler := NewConsoleLinkReconciler()
	test.EnsureReconcile(t, ctx, consoleLinkReconciler.Reconcile)

	consoleLink := &consolev1.ConsoleLink{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: defaults.GetConsoleLinkName()}, consoleLink)
	assert.Nil(t, err)
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
	assert.Equal(t, "https://test-host", consoleLink.Spec.Href)
	assert.Equal(t, fmt.Sprintf("https://test-host%s", defaults.GetConsoleLinkImage()), consoleLink.Spec.ApplicationMenu.ImageURL)
}

func TestDashboardRedirectURLMatchesDeploymentOverrides(t *testing.T) {
	value := func(url string) corev1.EnvVar { return corev1.EnvVar{Name: CheDashboardRedirectURLEnv, Value: url} }
	source := corev1.EnvVar{Name: CheDashboardRedirectURLEnv, ValueFrom: &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "redirect"}, Key: "url"},
	}}
	for _, env := range [][]corev1.EnvVar{
		{value("https://first.example"), value("https://last.example")},
		{value("https://first.example"), source},
		{source, value("https://last.example")},
		{value("https://first.example"), value("")},
		{},
	} {
		cluster := &chev2.CheCluster{}
		cluster.Spec.Components.Dashboard.Deployment = &chev2.Deployment{Containers: []chev2.Container{
			{Name: "arbitrary-override-name", Env: env},
			{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{value("https://ignored.example")}},
		}}
		ctx := test.NewCtxBuilder().WithCheCluster(cluster).Build()
		deployment := &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: defaults.GetCheFlavor() + "-dashboard"}}},
		}}}
		if err := deploy.OverrideDeployment(ctx, deployment, cluster.Spec.Components.Dashboard.Deployment); err != nil {
			t.Fatal(err)
		}
		expected := ""
		for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
			if env.Name == CheDashboardRedirectURLEnv && env.ValueFrom == nil {
				expected = env.Value
			}
		}
		assert.Equal(t, expected, getDashboardRedirectURL(ctx))
	}
}

func TestDashboardRedirectURLValidation(t *testing.T) {
	testCases := []struct {
		value string
		valid bool
	}{
		{"", false},
		{"   ", false},
		{"\ufeffhttps://example.com/app\ufeff", true},
		{"\u0085https://example.com/app", false},
		{"https://example.com/\ufeffpath", false},
		{"https://example.com/$(TARGET)", false},
		{"https://example.com/$$", false},
		{" \thttps://example.com/app?x=1#section \n", true},
		{"http://", false},
		{"https://", false},
		{"https:///example.com", false},
		{"https:example.com", false},
		{"ftp://example.com", false},
		{"javascript:alert(1)", false},
		{"not-a-url", false},
		{"//example.com", false},
		{"https://:443/path", false},
		{"https://example.com:65536", false},
		{"https://example.com:abc", false},
		{"http://example.com:0/path", true},
		{"https://example.com:65535/path?foo=bar#section", true},
		{"http://[::1]:8080/app", true},
		{"http://[invalid]/", false},
		{"http://[192.0.2.1]/", false},
		{"https://ｃｈｅ-host/", false},
		{"http://999.999.999.999/", false},
		{"https://exa mple.com/", false},
		{"https://example.com\\path", false},
		{"https://che-host/", false},
		{"https://CHE-HOST:0443/index.html?x=1#x", false},
		{"https://che-host/a/../", false},
		{"https://che-host/%2e/index.html", false},
		{"https://che-host/dashboard", true},
		{"https://che-host/dashboard/", true},
		{"https://che-host/some-other-path", true},
		{"http://che-host/", true},
		{"https://che-host:8443/", true},
	}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			cluster := &chev2.CheCluster{}
			cluster.Spec.Components.Dashboard.Deployment = &chev2.Deployment{
				Containers: []chev2.Container{{Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: tc.value}}}},
			}
			ctx := test.NewCtxBuilder().WithCheCluster(cluster).Build()
			ctx.CheHost = "che-host"
			expected := ""
			if tc.valid {
				expected = strings.Trim(strings.TrimSpace(tc.value), "\ufeff")
			}
			assert.Equal(t, expected, getDashboardRedirectURL(ctx))
		})
	}
}

func reconcileConsoleLink(t *testing.T, deployment *chev2.Deployment) string {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				Dashboard: chev2.Dashboard{
					Deployment: deployment,
				},
			},
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()
	reconciler := NewConsoleLinkReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	consoleLink := &consolev1.ConsoleLink{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: defaults.GetConsoleLinkName()}, consoleLink)
	assert.Nil(t, err)
	return consoleLink.Spec.Href
}

func TestReconcileConsoleLinkWithDashboardRedirect(t *testing.T) {
	testCases := []struct {
		name       string
		deployment *chev2.Deployment
		expected   string
	}{
		{
			name: "valid redirect URL in dashboard container",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name: defaults.GetCheFlavor() + "-dashboard",
						Env: []corev1.EnvVar{
							{Name: CheDashboardRedirectURLEnv, Value: "https://alternative-dashboard.example.com"},
						},
					},
				},
			},
			expected: "https://alternative-dashboard.example.com",
		},
		{
			name:       "nil deployment falls back to CheHost",
			deployment: nil,
			expected:   "https://che-host",
		},
		{
			name: "empty containers falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{},
			},
			expected: "https://che-host",
		},
		{
			name: "empty env list falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "unrelated env var falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name: defaults.GetCheFlavor() + "-dashboard",
						Env:  []corev1.EnvVar{{Name: "UNRELATED_ENV_VAR", Value: "https://unrelated.example.com"}},
					},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "empty redirect URL falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: ""}}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "whitespace redirect URL falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "   "}}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "malformed redirect URL falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "not-a-valid-url"}}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "disallowed scheme falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "javascript:alert(1)"}}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "missing host falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "http://"}}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "first override container applied when second matches dashboard flavor",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: "unrelated-sidecar", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://first-override.example.com"}}},
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://second-override.example.com"}}},
				},
			},
			expected: "https://first-override.example.com",
		},
		{
			name: "first override container applied when neither matches dashboard flavor",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: "unrelated-sidecar-a", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://sidecar-a.example.com"}}},
					{Name: "unrelated-sidecar-b", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://sidecar-b.example.com"}}},
				},
			},
			expected: "https://sidecar-a.example.com",
		},
		{
			name: "duplicate env vars in container resolve to last occurrence",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name: defaults.GetCheFlavor() + "-dashboard",
						Env: []corev1.EnvVar{
							{Name: CheDashboardRedirectURLEnv, Value: "https://first.example.com"},
							{Name: CheDashboardRedirectURLEnv, Value: "https://second-override.example.com"},
						},
					},
				},
			},
			expected: "https://second-override.example.com",
		},
		{
			name: "ValueFrom safely falls back to CheHost",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name: defaults.GetCheFlavor() + "-dashboard",
						Env: []corev1.EnvVar{
							{
								Name: CheDashboardRedirectURLEnv,
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: "custom-secret"},
										Key:                  "redirect-url",
									},
								},
							},
						},
					},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "root of current che host is rejected as self-redirect loop",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://che-host/"}}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "root without slash of current che host is rejected as loop",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://che-host"}}},
				},
			},
			expected: "https://che-host",
		},
		{
			name: "dashboard application path on same host is allowed",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://che-host/dashboard"}}},
				},
			},
			expected: "https://che-host/dashboard",
		},
		{
			name: "dashboard application with trailing slash on same host is allowed",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://che-host/dashboard/"}}},
				},
			},
			expected: "https://che-host/dashboard/",
		},
		{
			name: "alternative path on same host is allowed",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://che-host/custom-console"}}},
				},
			},
			expected: "https://che-host/custom-console",
		},
		{
			name: "default port with dashboard application is allowed",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://che-host:443/dashboard"}}},
				},
			},
			expected: "https://che-host:443/dashboard",
		},
		{
			name: "different port on same host is allowed",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://che-host:8443/dashboard"}}},
				},
			},
			expected: "https://che-host:8443/dashboard",
		},
		{
			name: "url with port, query, and fragment",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "https://alternative.example.com:8443/custom?foo=bar#section"}}},
				},
			},
			expected: "https://alternative.example.com:8443/custom?foo=bar#section",
		},
		{
			name: "ipv4 address with port",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "http://192.168.1.100:8080/app"}}},
				},
			},
			expected: "http://192.168.1.100:8080/app",
		},
		{
			name: "ipv6 address with port",
			deployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{Name: defaults.GetCheFlavor() + "-dashboard", Env: []corev1.EnvVar{{Name: CheDashboardRedirectURLEnv, Value: "http://[::1]:8080/app"}}},
				},
			},
			expected: "http://[::1]:8080/app",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := reconcileConsoleLink(t, tc.deployment)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
