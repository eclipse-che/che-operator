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

package solver

// Authentication (oauth-proxy) handling for endpoints marked as `secure: true`.

import (
	"context"
	"fmt"
	"maps"
	"slices"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/reconciler/devworkspaceoauthclient"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	oauthProxyPort = 4180

	oauthProxyCallbackPath = "/oauth/callback"
)

// oauthProxyScriptTemplate is the oauth-proxy startup script.
// Arguments: listen port, upstream port, OAuth client ID.
const oauthProxyScriptTemplate = `COOKIE_SECRET=$(openssl rand -base64 32) && \
OPENSHIFT_SAR="{\"namespace\":\"${DEVWORKSPACE_NAMESPACE}\",\"resource\":\"pods\",\"verb\":\"get\"}" && \
/usr/bin/oauth-proxy \
  --http-address=:%d \
  --provider=openshift \
  --https-address= \
  --upstream=http://localhost:%d \
  --cookie-secret=${COOKIE_SECRET} \
  --cookie-secure=true \
  --pass-basic-auth=false \
  --skip-provider-button=true \
  --client-id=%s \
  --client-secret-file=/var/run/oauth-client/client-secret \
  --openshift-ca=/etc/pki/tls/cert.pem \
  --openshift-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
  --openshift-sar="${OPENSHIFT_SAR}"`

func (c *CheRoutingSolver) provisionOAuthProxyPodAdditions(
	objs *solvers.RoutingObjects,
	cheCluster *chev2.CheCluster,
	routing *dwo.DevWorkspaceRouting,
) error {
	// TODO support multiply
	// TODO kubernetes/oauth disabled
	// TODO move resource limits to constants
	// TODO check if proxy needed
	// TODO add probs
	// TODO resolve image
	// TODO args
	// TODO ports
	// TODO volumes
	upstreamPort, found := getOAuthProxyUpstreamPort(routing)
	if !found {
		// Secure endpoints are served through the Che gateway, which already authenticates the requests.
		return nil
	}

	image := "image-registry.openshift-image-registry.svc:5000/openshift/oauth-proxy:v4.4"
	script := fmt.Sprintf(
		oauthProxyScriptTemplate,
		oauthProxyPort,
		upstreamPort,
		devworkspaceoauthclient.GetOAuthClientName(),
	)

	oauthContainer := corev1.Container{
		Name:            "oauth-proxy",
		Image:           image,
		ImagePullPolicy: corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image)),
		Command:         []string{"/bin/bash", "-c"},
		Args:            []string{script},
		Env: []corev1.EnvVar{
			{
				Name:  "DEVWORKSPACE_NAMESPACE",
				Value: routing.Namespace,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "oauth-client-token",
				MountPath: devworkspaceoauthclient.ClientSecretMountPath,
				ReadOnly:  true,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.5"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("64Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.1"),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "oauth-proxy",
				ContainerPort: oauthProxyPort,
				Protocol:      "TCP",
			},
		},
	}

	// Even though DefaultMode is optional in Kubernetes, DevWorkspace Controller needs it to be explicitly defined.
	// 420 = 0644 = '-rw-r--r--'
	defaultMode := int32(420)
	objs.PodAdditions.Volumes = append(objs.PodAdditions.Volumes, corev1.Volume{
		Name: "oauth-client-token",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  devworkspaceoauthclient.DevWorkspaceOAuthClientSecret,
				DefaultMode: &defaultMode,
			},
		},
	})

	// TODO override defaults
	// TODO strip cpu limits

	objs.PodAdditions.Containers = append(objs.PodAdditions.Containers, oauthContainer)

	return nil
}

// addOAuthRedirectURIs registers the redirect URIs of the workspace secure endpoints in the OAuthClient.
// Must be called after the routes have been provisioned into objs.
func (c *CheRoutingSolver) addOAuthRedirectURIs(objs *solvers.RoutingObjects, routing *dwo.DevWorkspaceRouting) error {
	if !infrastructure.IsOpenShift() {
		return nil
	}

	uris := getOAuthRedirectURIs(routing, objs.Routes)
	if len(uris) == 0 {
		return nil
	}

	err := c.updateOAuthClientRedirectURIs(false, func(redirectURIs []string) ([]string, bool) {
		changed := false
		for _, uri := range uris {
			if !slices.Contains(redirectURIs, uri) {
				redirectURIs = append(redirectURIs, uri)
				changed = true
			}
		}

		return redirectURIs, changed
	})
	if err != nil {
		return fmt.Errorf("failed to add redirect URIs to OAuthClient %s: %w", devworkspaceoauthclient.GetOAuthClientName(), err)
	}

	return nil
}

// removeOAuthRedirectURIs unregisters the redirect URIs of the workspace secure endpoints from the OAuthClient.
// The redirect URIs are computed from the workspace routes, which still exist while the routing is being finalized.
func (c *CheRoutingSolver) removeOAuthRedirectURIs(routing *dwo.DevWorkspaceRouting) error {
	if !infrastructure.IsOpenShift() || !c.hasSecureEndpoint(routing) {
		return nil
	}

	routeList := &routev1.RouteList{}
	if err := c.client.List(
		context.TODO(),
		routeList,
		client.InNamespace(routing.Namespace),
		client.MatchingLabels{dwconstants.DevWorkspaceIDLabel: routing.Spec.DevWorkspaceId},
	); err != nil {
		return fmt.Errorf("failed to list Routes in namespace %s: %w", routing.Namespace, err)
	}

	uris := getOAuthRedirectURIs(routing, routeList.Items)
	if len(uris) == 0 {
		return nil
	}

	err := c.updateOAuthClientRedirectURIs(true, func(redirectURIs []string) ([]string, bool) {
		filtered := slices.DeleteFunc(slices.Clone(redirectURIs), func(uri string) bool {
			return slices.Contains(uris, uri)
		})

		return filtered, len(filtered) != len(redirectURIs)
	})
	if err != nil {
		return fmt.Errorf("failed to remove redirect URIs from OAuthClient %s: %w", devworkspaceoauthclient.GetOAuthClientName(), err)
	}

	return nil
}

// updateOAuthClientRedirectURIs applies mutate to the OAuthClient redirect URIs and updates the OAuthClient
// if anything changed. The OAuthClient is shared by all workspaces, so the update is retried on conflict.
// If ignoreNotFound is false, a missing OAuthClient is reported as an error.
func (c *CheRoutingSolver) updateOAuthClientRedirectURIs(
	ignoreNotFound bool,
	mutate func(redirectURIs []string) ([]string, bool),
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		oauthClient := &oauthv1.OAuthClient{}
		exists, err := c.clientWrapper.GetIgnoreNotFound(
			context.TODO(),
			client.ObjectKey{Name: devworkspaceoauthclient.GetOAuthClientName()},
			oauthClient,
		)
		if err != nil {
			return fmt.Errorf("failed to get OAuthClient %s: %w", devworkspaceoauthclient.GetOAuthClientName(), err)
		}

		if !exists {
			if ignoreNotFound {
				return nil
			}

			return fmt.Errorf("failed to find OAuthClient %s", devworkspaceoauthclient.GetOAuthClientName())
		}

		redirectURIs, changed := mutate(oauthClient.RedirectURIs)
		if !changed {
			return nil
		}

		oauthClient.RedirectURIs = redirectURIs

		if err := c.client.Update(context.TODO(), oauthClient); err != nil {
			return fmt.Errorf("failed to update OAuthClient %s: %w", oauthClient.Name, err)
		}

		return nil
	})
}

// getOAuthRedirectURIs returns the OAuth redirect URIs for the routes exposing the workspace secure endpoints.
func getOAuthRedirectURIs(routing *dwo.DevWorkspaceRouting, routes []routev1.Route) []string {
	secureEndpoints := map[string]bool{}
	for _, endpoints := range routing.Spec.Endpoints {
		for _, e := range endpoints {
			if e.Secure {
				secureEndpoints[e.Name] = true
			}
		}
	}

	var uris []string
	for _, route := range routes {
		if route.Spec.Host == "" || !secureEndpoints[route.Annotations[defaults.ConfigAnnotationEndpointName]] {
			continue
		}

		uri := fmt.Sprintf("https://%s%s", route.Spec.Host, oauthProxyCallbackPath)
		if !slices.Contains(uris, uri) {
			uris = append(uris, uri)
		}
	}

	return uris
}

// isOAuthProxyEndpoint returns true if the endpoint is exposed with a dedicated Route and must be
// authenticated by the oauth-proxy sidecar. Endpoints supporting URL rewrite are served through the Che gateway instead.
func isOAuthProxyEndpoint(e dwo.Endpoint) bool {
	return e.Secure &&
		dw.EndpointExposure(e.Exposure) == dw.PublicEndpointExposure &&
		e.Attributes.GetString(urlRewriteSupportedEndpointAttributeName, nil) != "true"
}

// getOAuthProxyUpstreamPort returns the target port of the endpoint the oauth-proxy forwards authenticated requests to.
// Components are iterated in a sorted order to get a stable result.
func getOAuthProxyUpstreamPort(routing *dwo.DevWorkspaceRouting) (int, bool) {
	for _, componentName := range slices.Sorted(maps.Keys(routing.Spec.Endpoints)) {
		for _, e := range routing.Spec.Endpoints[componentName] {
			if isOAuthProxyEndpoint(e) {
				return e.TargetPort, true
			}
		}
	}

	return 0, false
}

// hasSecureEndpoint returns true if at least one endpoint of the workspace is marked as secure.
func (c *CheRoutingSolver) hasSecureEndpoint(routing *dwo.DevWorkspaceRouting) bool {
	for _, endpoints := range routing.Spec.Endpoints {
		for _, e := range endpoints {
			if e.Secure {
				return true
			}
		}
	}

	return false
}
