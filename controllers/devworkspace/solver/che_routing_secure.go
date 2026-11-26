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
	"slices"
	"strconv"
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
	// oauthProxyPortAttributeName is the endpoint attribute overriding the port the
	// oauth-proxy sidecar listens on. When unset, oauthProxyDefaultPort is used.
	oauthProxyPortAttributeName = "oauthProxyPort"

	// oauthProxyDefaultPort is the port the oauth-proxy sidecar listens on when the endpoint
	// does not override it via the oauthProxyPort attribute. The application itself keeps
	// listening on the endpoint targetPort, which is the proxy's upstream port.
	oauthProxyDefaultPort = 4180

	oauthProxyContainerName         = "oauth-proxy"
	oauthProxyVolumeName            = "oauth-client-token"
	oauthProxyClientSecretMountPath = "/var/run/oauth-client"
	oauthProxyClientSecretFile      = "token"

	oauthProxyServiceAccountPrefix = "oauth-proxy-"

	oauthRedirectReferenceAnnotationPrefix = "serviceaccounts.openshift.io/oauth-redirectreference."

	oauthProxyMemoryLimit   = "128Mi"
	oauthProxyCPULimit      = "100m"
	oauthProxyMemoryRequest = "32Mi"
	oauthProxyCPURequest    = "20m"
)

// hasOAuthProxyEndpoint returns true if the routing has at least one endpoint that must be
// served by the oauth-proxy sidecar.
func (c *CheRoutingSolver) hasOAuthProxyEndpoint(routing *dwo.DevWorkspaceRouting) bool {
	if !infrastructure.IsOpenShiftOAuthEnabled() {
		return false
	}

	for _, endpoints := range routing.Spec.Endpoints {
		if slices.ContainsFunc(endpoints, isOAuthProxyEndpoint) {
			return true
		}
	}

	return false
}

func (c *CheRoutingSolver) provisionOAuthProxyPodAdditions(cheCtx *chetypes.CheContext, objs *solvers.RoutingObjects, routing *dwo.DevWorkspaceRouting) error {
	found, listenPort, upstreamPort, err := resolveOAuthProxyPorts(routing)
	if !found {
		return err
	}

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
  --client-id=%s \
  --client-secret-file=%s/%s \
  --scope=%s \
  --email-domains=* \
  --cookie-expire=%s \
  --openshift-ca=/etc/pki/tls/cert.pem \
  --openshift-ca=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
  --openshift-sar="${OPENSHIFT_SAR}"`,
		listenPort,
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
				ContainerPort: int32(listenPort),
				Protocol:      "TCP",
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/oauth/healthz",
					Port:   intstr.FromInt32(int32(listenPort)),
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
					Port:   intstr.FromInt32(int32(listenPort)),
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

	return nil
}

func (c *CheRoutingSolver) provisionOAuthProxyServiceAccount(
	cheCtx *chetypes.CheContext,
	objs *solvers.RoutingObjects,
	routing *dwo.DevWorkspaceRouting,
) error {
	proxyRoutes := getOAuthProxyRoutes(routing, objs.Routes)
	if len(proxyRoutes) > 0 {
		if err := c.syncOAuthProxyServiceAccount(cheCtx, routing, proxyRoutes); err != nil {
			return err
		}

		if err := c.syncOAuthProxyServiceAccountTokenSecret(cheCtx, routing); err != nil {
			return err
		}
	}

	return nil
}

func (c *CheRoutingSolver) syncOAuthProxyServiceAccount(
	cheCtx *chetypes.CheContext,
	routing *dwo.DevWorkspaceRouting,
	proxyRoutes map[string]string,
) error {
	redirectRefAnnotations := map[string]string{}
	for routeName, port := range proxyRoutes {
		reference := fmt.Sprintf(
			`{"kind":"OAuthRedirectReference","apiVersion":"v1","reference":{"kind":"Route","name":%q}}`,
			routeName,
		)
		redirectRefAnnotations[fmt.Sprintf("%s%s", oauthRedirectReferenceAnnotationPrefix, port)] = reference
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

	if utils.ContainsEntries(sa.Annotations, redirectRefAnnotations) {
		return nil
	}

	sa.Annotations = utils.NonNilMap(sa.Annotations)
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
//   - port - the port the sidecar listens on: the endpoint's oauthProxyPort attribute,
//     or oauthProxyDefaultPort when the attribute is unset
//   - upstreamPort - the port of the application behind the secure endpoint, i.e. the
//     endpoint targetPort
//
// At most one secure endpoint per workspace is currently supported - an error is returned
// when there are more. Components are iterated in a sorted order to get a stable result.
func resolveOAuthProxyPorts(routing *dwo.DevWorkspaceRouting) (found bool, port, upstreamPort int, err error) {
	for _, componentName := range slices.Sorted(maps.Keys(routing.Spec.Endpoints)) {
		for _, e := range routing.Spec.Endpoints[componentName] {
			if !isOAuthProxyEndpoint(e) {
				continue
			}

			if found {
				cause := fmt.Sprintf("only one secure endpoint per workspace is currently supported, found endpoint %s/%s", componentName, e.Name)
				return false, 0, 0, oauthProxyErrorf(routing, cause)
			}

			portStr := e.Attributes.GetString(oauthProxyPortAttributeName, nil)
			if portStr == "" {
				port = oauthProxyDefaultPort
			} else {
				port, err = strconv.Atoi(portStr)
				if err != nil {
					cause := fmt.Sprintf("endpoint %s/%s: '%s' attribute must be an integer", componentName, e.Name, oauthProxyPortAttributeName)
					return false, 0, 0, oauthProxyErrorf(routing, cause)
				}
			}

			if port == e.TargetPort {
				cause := fmt.Sprintf("endpoint %s/%s: '%s' must differ from the endpoint targetPort %d the oauth-proxy forwards requests to", componentName, e.Name, oauthProxyPortAttributeName, e.TargetPort)
				return false, 0, 0, oauthProxyErrorf(routing, cause)
			}

			found = true
			upstreamPort = e.TargetPort
		}
	}

	return found, port, upstreamPort, nil
}

// getOAuthProxyListenPort returns the port the oauth-proxy sidecar listens on for the given
// secure endpoint: the oauthProxyPort attribute when set to a valid port, the default
// otherwise. Unlike resolveOAuthProxyPorts it never fails - use it only where the attribute
// has already been validated or where falling back to the default is acceptable.
func getOAuthProxyListenPort(e dwo.Endpoint) int {
	if port, err := strconv.Atoi(e.Attributes.GetString(oauthProxyPortAttributeName, nil)); err == nil && port > 0 {
		return port
	}

	return oauthProxyDefaultPort
}

// endpointExposedPort returns the port the given endpoint is exposed on outside the
// workspace pod: the oauth-proxy listen port for secure endpoints served by the sidecar,
// the endpoint targetPort for all the others.
func endpointExposedPort(e dwo.Endpoint) int32 {
	if isOAuthProxyEndpoint(e) {
		return int32(getOAuthProxyListenPort(e))
	}

	return int32(e.TargetPort)
}

func oauthProxyErrorf(routing *dwo.DevWorkspaceRouting, cause string) error {
	return fmt.Errorf(
		"failed to configure oauth-proxy for DevWorkspace %s in namespace %s: %s",
		routing.Spec.DevWorkspaceId, routing.Namespace, cause)
}

func getOAuthProxyServiceAccountName(routing *dwo.DevWorkspaceRouting) string {
	return oauthProxyServiceAccountPrefix + routing.Spec.DevWorkspaceId
}

func getOAuthProxyTokenSecretName(routing *dwo.DevWorkspaceRouting) string {
	return getOAuthProxyServiceAccountName(routing) + "-token"
}

// isOAuthProxyEndpoint returns true if the endpoint is exposed with a dedicated Route
// and must be authenticated by the oauth-proxy sidecar.
func isOAuthProxyEndpoint(e dwo.Endpoint) bool {
	return e.Secure &&
		dw.EndpointExposure(e.Exposure) == dw.PublicEndpointExposure &&
		e.Attributes.GetString(urlRewriteSupportedEndpointAttributeName, nil) != "true"
}
