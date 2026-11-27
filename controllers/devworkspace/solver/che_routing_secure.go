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

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	dwdefaults "github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// oauthProxyPortAttributeName is the endpoint attribute setting the port the oauth-proxy
	// sidecar listens on. It is required for a secure endpoint to be served by the oauth-proxy:
	// when unset, the endpoint is exposed without authentication, as before the oauth-proxy
	// support, so that existing workspaces with secure endpoints keep working unchanged.
	oauthProxyPortAttributeName = "oauthProxyPort"

	oauthProxyContainerName         = "oauth-proxy"
	oauthProxyVolumeName            = "oauth-client-token"
	oauthProxyClientSecretMountPath = "/var/run/oauth-client"
	oauthProxyClientSecretFile      = "token"

	oauthProxyServiceAccountPrefix = "oauth-proxy-"

	// oauthProxyServicePortSuffix is appended to the endpoint name to name the Service port
	// exposing the oauth-proxy listen port; the bare endpoint name is kept for the port
	// in-cluster clients discovered, which is forwarded to the oauth-proxy as well
	oauthProxyServicePortSuffix = "-oauth-proxy"

	oauthRedirectReferenceAnnotationPrefix = "serviceaccounts.openshift.io/oauth-redirectreference."

	oauthProxyMemoryLimit   = "128Mi"
	oauthProxyCPULimit      = "100m"
	oauthProxyMemoryRequest = "32Mi"
	oauthProxyCPURequest    = "20m"
)

// provisionOAuthProxyPodAdditions adds the oauth-proxy sidecar to the workspace pod. It listens
// on oauthProxyPort and forwards authenticated requests to the application on upstreamPort.
// Both ports must have been resolved and validated by resolveOAuthProxyPorts.
func (c *CheRoutingSolver) provisionOAuthProxyPodAdditions(
	cheCtx *chetypes.CheContext,
	objs *solvers.RoutingObjects,
	routing *dwo.DevWorkspaceRouting,
	oauthProxyPort int32,
	upstreamPort int32,
) {
	image := gateway.GetOpenShiftOAuthProxyImage(cheCtx)

	scope := "user:info user:check-access"
	cookieExpire := (time.Duration(constants.DefaultOAuthProxyCookieExpireSeconds) * time.Second).String()
	args := fmt.Sprintf(`COOKIE_SECRET=$(openssl rand -base64 32) && \
DEVWORKSPACE_NAMESPACE=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace) && \
OPENSHIFT_SAR="{\"namespace\":\"${DEVWORKSPACE_NAMESPACE}\",\"verb\":\"get\",\"group\":\"workspace.devfile.io\",\"resource\":\"devworkspaces\"}" && \
/usr/bin/oauth-proxy \
  --http-address=:%d \
  --provider=openshift \
  --https-address= \
  --upstream=http://localhost:%d \
  --cookie-secret=${COOKIE_SECRET} \
  --cookie-secure=true \
  --pass-basic-auth=false \
  --skip-provider-button=true \
  --client-id='%s' \
  --client-secret-file='%s/%s' \
  --scope='%s' \
  --email-domain='*' \
  --cookie-expire='%s' \
  --openshift-ca=/etc/pki/tls/cert.pem \
  --openshift-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
  --openshift-sar="${OPENSHIFT_SAR}"`,
		oauthProxyPort,
		upstreamPort,
		"system:serviceaccount:"+routing.Namespace+":"+getOAuthProxyServiceAccountName(routing),
		oauthProxyClientSecretMountPath,
		oauthProxyClientSecretFile,
		scope,
		cookieExpire,
	)

	container := corev1.Container{
		Name:            oauthProxyContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image)),
		Command:         []string{"/bin/bash", "-c"},
		Args:            []string{args},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      oauthProxyVolumeName,
				MountPath: oauthProxyClientSecretMountPath,
				ReadOnly:  true,
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(oauthProxyMemoryLimit),
				corev1.ResourceCPU:    resource.MustParse(oauthProxyCPULimit),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(oauthProxyMemoryRequest),
				corev1.ResourceCPU:    resource.MustParse(oauthProxyCPURequest),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          oauthProxyContainerName,
				ContainerPort: oauthProxyPort,
				Protocol:      "TCP",
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/oauth/healthz",
					Port:   intstr.FromInt32(oauthProxyPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			TimeoutSeconds:      5,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/oauth/healthz",
					Port:   intstr.FromInt32(oauthProxyPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      5,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
		},
	}
	objs.PodAdditions.Containers = append(objs.PodAdditions.Containers, container)

	volume := corev1.Volume{
		Name: oauthProxyVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  getOAuthProxyTokenSecretName(routing),
				DefaultMode: new(int32(420)),
			},
		},
	}
	objs.PodAdditions.Volumes = append(objs.PodAdditions.Volumes, volume)
}

func (c *CheRoutingSolver) provisionOAuthProxyServiceAccount(
	cheCtx *chetypes.CheContext,
	objs *solvers.RoutingObjects,
	routing *dwo.DevWorkspaceRouting,
) error {
	// the oauth-proxy sidecar mounts the token Secret of the ServiceAccount, so the routing must
	// not be reported as ready without them, otherwise the workspace pod hangs on a missing volume
	proxyRoutes := getOAuthProxyRoutes(routing, objs.Routes)
	if len(proxyRoutes) == 0 {
		return &solvers.RoutingNotReady{Retry: 5 * time.Second}
	}

	if err := c.syncOAuthProxyServiceAccount(cheCtx, routing, proxyRoutes); err != nil {
		return err
	}

	return c.syncOAuthProxyServiceAccountTokenSecret(cheCtx, routing)
}

func (c *CheRoutingSolver) syncOAuthProxyServiceAccount(
	cheCtx *chetypes.CheContext,
	routing *dwo.DevWorkspaceRouting,
	proxyRoutes map[string]string,
) error {
	redirectRefAnnotations := map[string]string{}
	for routeName, suffix := range proxyRoutes {
		reference := fmt.Sprintf(
			`{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":%q}}`,
			routeName,
		)
		redirectRefAnnotations[fmt.Sprintf("%s%s", oauthRedirectReferenceAnnotationPrefix, suffix)] = reference
	}

	sa := &corev1.ServiceAccount{}
	exists, err := resources.ServiceAccount.Get(cheCtx, getOAuthProxyServiceAccountName(routing), routing.Namespace, sa)
	if err != nil {
		return err
	}

	if !exists {
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getOAuthProxyServiceAccountName(routing),
				Namespace: routing.Namespace,
				Labels: map[string]string{
					dwconstants.DevWorkspaceIDLabel:    routing.Spec.DevWorkspaceId,
					constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
				},
				Annotations: redirectRefAnnotations,
			},
		}

		if err := controllerutil.SetControllerReference(routing, sa, cheCtx.ClusterAPI.Scheme); err != nil {
			return err
		}
		return resources.ServiceAccount.Create(cheCtx, sa)
	}

	sa.Annotations = utils.NonNilMap(sa.Annotations)
	maps.DeleteFunc(sa.Annotations, func(key string, _ string) bool {
		return strings.HasPrefix(key, oauthRedirectReferenceAnnotationPrefix)
	})
	maps.Copy(sa.Annotations, redirectRefAnnotations)

	if err := controllerutil.SetControllerReference(routing, sa, cheCtx.ClusterAPI.Scheme); err != nil {
		return err
	}
	return resources.ServiceAccount.Sync(cheCtx, sa)
}

func (c *CheRoutingSolver) syncOAuthProxyServiceAccountTokenSecret(
	cheCtx *chetypes.CheContext,
	routing *dwo.DevWorkspaceRouting,
) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getOAuthProxyTokenSecretName(routing),
			Namespace: routing.Namespace,
			Labels: map[string]string{
				dwconstants.DevWorkspaceIDLabel:    routing.Spec.DevWorkspaceId,
				constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
			},
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": getOAuthProxyServiceAccountName(routing),
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	if err := controllerutil.SetControllerReference(routing, secret, cheCtx.ClusterAPI.Scheme); err != nil {
		return err
	}

	if err := client.IgnoreAlreadyExists(resources.Secret.Create(cheCtx, secret)); err != nil {
		return err
	}

	return nil
}

// getOAuthProxyRoutes returns the Routes exposing the workspace secure endpoints served by the
// oauth-proxy sidecar, as a map of the Route name to the port the Route targets.
func getOAuthProxyRoutes(routing *dwo.DevWorkspaceRouting, routes []routev1.Route) map[string]string {
	secureEndpoints := map[string]bool{}
	for _, endpoints := range routing.Spec.Endpoints {
		for _, e := range endpoints {
			if isOAuthProxyEndpoint(e) {
				secureEndpoints[e.Name] = true
			}
		}
	}

	proxyRoutes := map[string]string{}
	for _, route := range routes {
		if secureEndpoints[route.Annotations[dwdefaults.ConfigAnnotationEndpointName]] {
			proxyRoutes[route.Name] = route.Spec.Port.TargetPort.String()
		}
	}

	return proxyRoutes
}

// resolveOAuthProxyPorts returns whether the workspace pod needs the oauth-proxy sidecar,
// and if so, the two ports it is configured with:
//   - oauthProxyPort - the port the sidecar listens on, taken from the endpoint's
//     oauthProxyPort attribute
//   - upstreamPort - the port of the application behind the secure endpoint, i.e. the
//     endpoint targetPort
func resolveOAuthProxyPorts(
	routing *dwo.DevWorkspaceRouting,
) (found bool, oauthProxyPort, upstreamPort int32, err error) {
	for _, componentName := range slices.Sorted(maps.Keys(routing.Spec.Endpoints)) {
		for _, e := range routing.Spec.Endpoints[componentName] {
			if !isOAuthProxyEndpoint(e) {
				continue
			}

			if found {
				cause := fmt.Sprintf("only one secure endpoint per workspace is currently supported, found endpoint %s/%s", componentName, e.Name)
				return false, 0, 0, oauthProxyErrorf(routing, cause)
			}

			port, err := parseOAuthProxyPort(e)
			if err != nil {
				cause := fmt.Sprintf("endpoint %s/%s: %v", componentName, e.Name, err)
				return false, 0, 0, oauthProxyErrorf(routing, cause)
			}

			if cause := findOAuthProxyPortConflict(routing, int(port)); cause != "" {
				cause = fmt.Sprintf("endpoint %s/%s: '%s' %d %s", componentName, e.Name, oauthProxyPortAttributeName, port, cause)
				return false, 0, 0, oauthProxyErrorf(routing, cause)
			}

			if cause := findTargetPortConflict(routing, componentName, e); cause != "" {
				cause = fmt.Sprintf("endpoint %s/%s: targetPort %d %s", componentName, e.Name, e.TargetPort, cause)
				return false, 0, 0, oauthProxyErrorf(routing, cause)
			}

			found = true
			oauthProxyPort = port
			upstreamPort = int32(e.TargetPort)
		}
	}

	return found, oauthProxyPort, upstreamPort, nil
}

func parseOAuthProxyPort(e dwo.Endpoint) (int32, error) {
	value := e.Attributes.GetString(oauthProxyPortAttributeName, nil)

	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > math.MaxUint16 {
		return 0, &solvers.RoutingInvalid{
			Reason: fmt.Sprintf("'%s' attribute must be a port number between 1 and %d, got '%s'", oauthProxyPortAttributeName, math.MaxUint16, value),
		}
	}

	return int32(port), nil
}

// findOAuthProxyPortConflict returns the reason why the oauth-proxy sidecar can't listen on
// the given port, or an empty string when the port is free.
func findOAuthProxyPortConflict(routing *dwo.DevWorkspaceRouting, oauthProxyPort int) string {
	if oauthProxyPort == wsGatewayPort {
		return fmt.Sprintf("must differ from the port %d of the workspace gateway", wsGatewayPort)
	}

	for _, componentName := range slices.Sorted(maps.Keys(routing.Spec.Endpoints)) {
		for _, e := range routing.Spec.Endpoints[componentName] {
			if e.TargetPort == oauthProxyPort {
				return fmt.Sprintf("must differ from the targetPort of the endpoint %s/%s", componentName, e.Name)
			}
		}
	}

	return ""
}

// findTargetPortConflict returns the reason why the secure endpoint can't be served by the
// oauth-proxy sidecar because another exposed endpoint shares its targetPort, or an empty
// string when there is no such endpoint. Such an endpoint would expose the targetPort on the
// common Service straight to the application, so in-cluster clients could bypass the oauth-proxy.
func findTargetPortConflict(
	routing *dwo.DevWorkspaceRouting,
	secureComponentName string,
	secureEndpoint dwo.Endpoint,
) string {
	for _, componentName := range slices.Sorted(maps.Keys(routing.Spec.Endpoints)) {
		for _, e := range routing.Spec.Endpoints[componentName] {
			if componentName == secureComponentName && e.Name == secureEndpoint.Name {
				continue
			}

			if dw.EndpointExposure(e.Exposure) == dw.NoneEndpointExposure {
				continue
			}

			if e.TargetPort == secureEndpoint.TargetPort {
				return fmt.Sprintf("is shared with the endpoint %s/%s", componentName, e.Name)
			}
		}
	}

	return ""
}

// endpointExposedPort returns the port the given endpoint is exposed on outside the
// workspace pod: the oauth-proxy listen port for secure endpoints served by the sidecar,
// the endpoint targetPort for all the others.
func endpointExposedPort(e dwo.Endpoint) (int32, error) {
	if isOAuthProxyEndpoint(e) {
		return parseOAuthProxyPort(e)
	}

	return int32(e.TargetPort), nil
}

func oauthProxyErrorf(routing *dwo.DevWorkspaceRouting, cause string) error {
	return &solvers.RoutingInvalid{
		Reason: fmt.Sprintf("failed to configure oauth-proxy for DevWorkspace %s in namespace %s: %s",
			routing.Spec.DevWorkspaceId, routing.Namespace, cause),
	}
}

func getOAuthProxyServiceAccountName(routing *dwo.DevWorkspaceRouting) string {
	return oauthProxyServiceAccountPrefix + routing.Spec.DevWorkspaceId
}

func getOAuthProxyTokenSecretName(routing *dwo.DevWorkspaceRouting) string {
	return getOAuthProxyServiceAccountName(routing) + "-token"
}

// isOAuthProxyEndpoint returns true if the endpoint is exposed with a dedicated Route
// and must be authenticated by the oauth-proxy sidecar, i.e. when all of the following hold:
//   - the endpoint is secure and public
//   - OpenShift OAuth is enabled
//   - the endpoint sets the oauthProxyPort attribute
//   - the endpoint is not served through the gateway (urlRewriteSupported is not "true")
func isOAuthProxyEndpoint(e dwo.Endpoint) bool {
	return e.Secure &&
		infrastructure.IsOpenShiftOAuthEnabled() &&
		dw.EndpointExposure(e.Exposure) == dw.PublicEndpointExposure &&
		e.Attributes.GetString(oauthProxyPortAttributeName, nil) != "" &&
		e.Attributes.GetString(urlRewriteSupportedEndpointAttributeName, nil) != "true"
}
