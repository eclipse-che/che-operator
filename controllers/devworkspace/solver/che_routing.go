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
	"path"
	"regexp"
	"strings"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"

	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/common"
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	dwdefaults "github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/sync"
	"github.com/google/go-cmp/cmp/cmpopts"
	routeV1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	uniqueEndpointAttributeName              = "unique"
	urlRewriteSupportedEndpointAttributeName = "urlRewriteSupported"
	endpointURLPrefixPattern                 = "/%s/%s/%d"
	// note - che-theia DEPENDS on this format - we should not change this unless crosschecked with the che-theia impl
	uniqueEndpointURLPrefixPattern = "/%s/%s/%s"
	wsGatewayPort                  = 3030
	wsGatewayName                  = "che-gateway"
)

var (
	configMapDiffOpts = cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta")
)

func (c *CheRoutingSolver) cheSpecObjects(cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, workspaceMeta solvers.DevWorkspaceMetadata) (solvers.RoutingObjects, error) {
	objs := solvers.RoutingObjects{}

	if err := c.provisionServices(&objs, cheCluster, routing, workspaceMeta); err != nil {
		return solvers.RoutingObjects{}, err
	}

	if err := c.provisionRouting(&objs, cheCluster, routing, workspaceMeta); err != nil {
		return solvers.RoutingObjects{}, err
	}

	if err := c.provisionPodAdditions(&objs, cheCluster, routing); err != nil {
		return solvers.RoutingObjects{}, err
	}

	return objs, nil
}

func (c *CheRoutingSolver) provisionServices(objs *solvers.RoutingObjects, cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, workspaceMeta solvers.DevWorkspaceMetadata) error {
	objs.Services = solvers.GetDiscoverableServicesForEndpoints(routing.Spec.Endpoints, workspaceMeta)

	commonService := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      common.ServiceName(routing.Spec.DevWorkspaceId),
			Namespace: routing.Namespace,
			Labels: map[string]string{
				dwconstants.DevWorkspaceIDLabel:    routing.Spec.DevWorkspaceId,
				constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: routing.Spec.PodSelector,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       common.EndpointName("ws-route"),
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(wsGatewayPort),
					TargetPort: intstr.FromInt(wsGatewayPort),
				},
			},
		},
	}
	objs.Services = append(objs.Services, *commonService)

	annos := map[string]string{}
	annos[dwdefaults.ConfigAnnotationCheManagerName] = cheCluster.Name
	annos[dwdefaults.ConfigAnnotationCheManagerNamespace] = cheCluster.Namespace

	additionalLabels := dwdefaults.GetLabelsForComponent(cheCluster, "exposure")

	for i := range objs.Services {
		// need to use a ref otherwise s would be a copy
		s := &objs.Services[i]

		if s.Labels == nil {
			s.Labels = map[string]string{}
		}

		for k, v := range additionalLabels {

			if len(s.Labels[k]) == 0 {
				s.Labels[k] = v
			}
		}

		if s.Annotations == nil {
			s.Annotations = map[string]string{}
		}

		for k, v := range annos {

			if len(s.Annotations[k]) == 0 {
				s.Annotations[k] = v
			}
		}
	}

	return nil
}

func (c *CheRoutingSolver) provisionRouting(objs *solvers.RoutingObjects, cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, workspaceMeta solvers.DevWorkspaceMetadata) error {
	// k, now we have to create our own objects for configuring the gateway
	configMaps, err := c.getGatewayConfigsAndFillRoutingObjects(cheCluster, workspaceMeta.DevWorkspaceId, routing, objs)
	if err != nil {
		return err
	}

	// solvers.RoutingObjects does not currently support ConfigMaps, so we have to actually create it in cluster
	syncer := sync.New(c.client, c.scheme)
	for _, cm := range configMaps {
		_, _, err := syncer.Sync(context.TODO(), nil, &cm, configMapDiffOpts)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *CheRoutingSolver) provisionPodAdditions(objs *solvers.RoutingObjects, cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting) error {
	objs.PodAdditions = &dwo.PodAdditions{
		Containers: []corev1.Container{},
		Volumes:    []corev1.Volume{},
	}

	image := defaults.GetGatewayImage(cheCluster)
	if cheCluster.Spec.Networking.Auth.Gateway.Deployment != nil {
		for _, c := range cheCluster.Spec.Networking.Auth.Gateway.Deployment.Containers {
			if c.Name == constants.GatewayContainerName {
				image = utils.GetValue(c.Image, image)
			}
		}
	}

	gatewayContainer := &corev1.Container{
		Name:            wsGatewayName,
		Image:           image,
		ImagePullPolicy: corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image)),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      wsGatewayName,
				MountPath: "/etc/traefik",
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(constants.DefaultGatewayMemoryLimit),
				corev1.ResourceCPU:    resource.MustParse(constants.DefaultGatewayCpuLimit),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse(constants.DefaultGatewayMemoryRequest),
				corev1.ResourceCPU:    resource.MustParse(constants.DefaultGatewayCpuRequest),
			},
		},
	}

	if err := deploy.OverrideContainer(routing.GetNamespace(), gatewayContainer, cheCluster.Spec.DevEnvironments.GatewayContainer); err != nil {
		return err
	}

	objs.PodAdditions.Containers = append(objs.PodAdditions.Containers, *gatewayContainer)

	// Even though DefaultMode is optional in Kubernetes, DevWorkspace Controller needs it to be explicitly defined.
	// 420 = 0644 = '-rw-r--r--'
	defaultMode := int32(420)
	objs.PodAdditions.Volumes = append(objs.PodAdditions.Volumes, corev1.Volume{
		Name: wsGatewayName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: dwdefaults.GetGatewayWorkspaceConfigMapName(routing.Spec.DevWorkspaceId),
				},
				DefaultMode: &defaultMode,
			},
		},
	})

	return nil
}

func (c *CheRoutingSolver) cheExposedEndpoints(cheCluster *chev2.CheCluster, workspaceID string, componentEndpoints map[string]dwo.EndpointList, routingObj solvers.RoutingObjects) (exposedEndpoints map[string]dwo.ExposedEndpointList, ready bool, err error) {
	if cheCluster.Status.GatewayPhase == chev2.GatewayPhaseInitializing {
		return nil, false, nil
	}

	exposedEndpoints = map[string]dwo.ExposedEndpointList{}

	gatewayHost := cheCluster.GetCheHost()

	endpointStrategy := getEndpointPathStrategy(c.client, workspaceID, routingObj.Services[0].Namespace, routingObj.Services[0].ObjectMeta.OwnerReferences[0].Name)

	for component, endpoints := range componentEndpoints {
		for _, endpoint := range endpoints {
			if dw.EndpointExposure(endpoint.Exposure) == dw.NoneEndpointExposure {
				continue
			}

			scheme := determineEndpointScheme(endpoint)

			if !isExposableScheme(scheme) {
				// we cannot expose non-http endpoints publicly, because ingresses/routes only support http(s)
				continue
			}

			// The gateway server must not be set up
			// Endpoint url is set to the service hostname
			if dw.EndpointExposure(endpoint.Exposure) == dw.InternalEndpointExposure {
				internalUrl := getServiceURL(int32(endpoint.TargetPort), workspaceID, routingObj.Services[0].Namespace)
				exposedEndpoints[component] = append(exposedEndpoints[component], dwo.ExposedEndpoint{
					Name:       endpoint.Name,
					Url:        internalUrl,
					Attributes: endpoint.Attributes,
				})

				continue
			}

			// try to find the endpoint in the ingresses/routes first. If it is there, it is exposed on a subdomain
			// otherwise it is exposed through the gateway
			var endpointURL string
			if infrastructure.IsOpenShift() {
				route := findRouteForEndpoint(component, endpoint, &routingObj, workspaceID)
				if route != nil {
					endpointURL = path.Join(route.Spec.Host, endpoint.Path)
				}
			} else {
				ingress := findIngressForEndpoint(component, endpoint, &routingObj)
				if ingress != nil {
					endpointURL = path.Join(ingress.Spec.Rules[0].Host, endpoint.Path)
				}
			}

			if endpointURL == "" {
				if gatewayHost == "" {
					// the gateway has not yet established the host
					return map[string]dwo.ExposedEndpointList{}, false, nil
				}

				publicURLPrefix := getPublicURLPrefixForEndpoint(component, endpoint, endpointStrategy)
				endpointURL = path.Join(gatewayHost, publicURLPrefix, endpoint.Path)
			}

			publicURL := scheme + "://" + endpointURL

			// path.Join() removes the trailing slashes, so make sure to reintroduce that if required.
			if endpoint.Path == "" || strings.HasSuffix(endpoint.Path, "/") {
				publicURL = publicURL + "/"
			}

			exposedEndpoints[component] = append(exposedEndpoints[component], dwo.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        publicURL,
				Attributes: endpoint.Attributes,
			})
		}
	}

	return exposedEndpoints, true, nil
}

func isExposableScheme(scheme string) bool {
	return strings.HasPrefix(scheme, "http") || strings.HasPrefix(scheme, "ws")
}

func secureScheme(scheme string) string {
	if scheme == "http" {
		return "https"
	} else if scheme == "ws" {
		return "wss"
	} else {
		return scheme
	}
}

func isSecureScheme(scheme string) bool {
	return scheme == "https" || scheme == "wss"
}

func (c *CheRoutingSolver) getGatewayConfigsAndFillRoutingObjects(cheCluster *chev2.CheCluster, workspaceID string, routing *dwo.DevWorkspaceRouting, objs *solvers.RoutingObjects) ([]corev1.ConfigMap, error) {
	restrictedAnno, setRestrictedAnno := routing.Annotations[dwconstants.DevWorkspaceRestrictedAccessAnnotation]

	cmLabels := dwdefaults.AddStandardLabelsForComponent(cheCluster, "gateway-config", dwdefaults.GetGatewayWorkspaceConfigMapLabels(cheCluster))
	cmLabels[dwconstants.DevWorkspaceIDLabel] = workspaceID
	if setRestrictedAnno {
		cmLabels[dwconstants.DevWorkspaceRestrictedAccessAnnotation] = restrictedAnno
	}

	configs := make([]corev1.ConfigMap, 0)
	endpointStrategy := getEndpointPathStrategy(c.client, workspaceID, routing.Namespace, routing.Name)

	// first do routing from main che-gateway into workspace service
	if mainWsRouteConfig, err := provisionMainWorkspaceRoute(cheCluster, routing, cmLabels, endpointStrategy); err != nil {
		return nil, err
	} else {
		configs = append(configs, *mainWsRouteConfig)
	}

	// then expose the endpoints
	if infraExposer, err := c.getInfraSpecificExposer(cheCluster, routing, objs, endpointStrategy); err != nil {
		return nil, err
	} else {
		if workspaceConfig := exposeAllEndpoints(cheCluster, routing, objs, infraExposer, endpointStrategy); workspaceConfig != nil {
			configs = append(configs, *workspaceConfig)
		}
	}

	return configs, nil
}

func getEndpointPathStrategy(c client.Client, workspaceId string, namespace string, dwRoutingName string) EndpointStrategy {
	useLegacyPaths := false
	username, err := getNormalizedUsername(c, namespace)
	if err != nil {
		useLegacyPaths = true
	}

	dwName, err := getNormalizedWkspName(c, namespace, dwRoutingName)
	if err != nil {
		useLegacyPaths = true
	}

	if useLegacyPaths {
		strategy := new(Legacy)
		strategy.workspaceID = workspaceId
		return strategy
	} else {
		strategy := new(UsernameWkspName)
		strategy.username = username
		strategy.workspaceName = dwName
		strategy.workspaceID = workspaceId
		return strategy
	}
}

func getNormalizedUsername(c client.Client, namespace string) (string, error) {
	username, err := getUsernameFromNamespace(c, namespace)

	if err != nil {
		username, err = getUsernameFromSecret(c, namespace)
	}

	if err != nil {
		return "", err
	}
	return normalize(username), nil
}

func getUsernameFromSecret(c client.Client, namespace string) (string, error) {
	secret := &corev1.Secret{}
	err := c.Get(context.TODO(), client.ObjectKey{Name: "user-profile", Namespace: namespace}, secret)
	if err != nil {
		return "", err
	}
	username := string(secret.Data["name"])
	return normalize(username), nil
}

func getUsernameFromNamespace(c client.Client, namespace string) (string, error) {
	nsInfo := &corev1.Namespace{}
	err := c.Get(context.TODO(), client.ObjectKey{Name: namespace}, nsInfo)
	if err != nil {
		return "", err
	}

	notFoundError := fmt.Errorf("username not found in namespace %s", namespace)

	if nsInfo.GetAnnotations() == nil {
		return "", notFoundError
	}

	username, exists := nsInfo.GetAnnotations()["che.eclipse.org/username"]

	if exists {
		return username, nil
	}

	return "", notFoundError
}

func getNormalizedWkspName(c client.Client, namespace string, routingName string) (string, error) {
	routing := &dwo.DevWorkspaceRouting{}
	err := c.Get(context.TODO(), client.ObjectKey{Name: routingName, Namespace: namespace}, routing)
	if err != nil {
		return "", err
	}
	return normalize(routing.ObjectMeta.OwnerReferences[0].Name), nil
}

func normalize(username string) string {
	r1 := regexp.MustCompile(`[^-a-zA-Z0-9]`)
	r2 := regexp.MustCompile(`-+`)
	r3 := regexp.MustCompile(`^-|-$`)

	result := r1.ReplaceAllString(username, "-") // replace invalid chars with '-'
	result = r2.ReplaceAllString(result, "-")    // replace multiple '-' with single ones
	result = r3.ReplaceAllString(result, "")     // trim dashes at beginning/end
	return strings.ToLower(result)
}

func (c *CheRoutingSolver) getInfraSpecificExposer(cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, objs *solvers.RoutingObjects, endpointStrategy EndpointStrategy) (func(info *EndpointInfo), error) {
	if infrastructure.IsOpenShift() {
		exposer := &RouteExposer{}
		if err := exposer.initFrom(context.TODO(), c.client, cheCluster, routing); err != nil {
			return nil, err
		}
		return func(info *EndpointInfo) {
			route := exposer.getRouteForService(info, endpointStrategy)
			objs.Routes = append(objs.Routes, route)
		}, nil
	} else {
		exposer := &IngressExposer{}
		if err := exposer.initFrom(context.TODO(), c.client, cheCluster, routing, dwdefaults.GetIngressAnnotations(cheCluster)); err != nil {
			return nil, err
		}
		return func(info *EndpointInfo) {
			ingress := exposer.getIngressForService(info, endpointStrategy)
			objs.Ingresses = append(objs.Ingresses, ingress)
		}, nil
	}
}

func getCommonService(objs *solvers.RoutingObjects, dwId string) *corev1.Service {
	commonServiceName := common.ServiceName(dwId)
	for i, svc := range objs.Services {
		if svc.Name == commonServiceName {
			return &objs.Services[i]
		}
	}
	return nil
}

func exposeAllEndpoints(cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, objs *solvers.RoutingObjects, ingressExpose func(*EndpointInfo), endpointStrategy EndpointStrategy) *corev1.ConfigMap {
	wsRouteConfig := gateway.CreateEmptyTraefikConfig()

	commonService := getCommonService(objs, routing.Spec.DevWorkspaceId)
	if commonService == nil {
		return nil
	}

	order := 1
	for componentName, endpoints := range routing.Spec.Endpoints {
		for _, e := range endpoints {
			if dw.EndpointExposure(e.Exposure) == dw.NoneEndpointExposure {
				continue
			}

			if e.Attributes.GetString(urlRewriteSupportedEndpointAttributeName, nil) != "true" {
				if !containPort(commonService, int32(e.TargetPort)) {
					commonService.Spec.Ports = append(commonService.Spec.Ports, corev1.ServicePort{
						Name:       common.EndpointName(e.Name),
						Protocol:   corev1.ProtocolTCP,
						Port:       int32(e.TargetPort),
						TargetPort: intstr.FromInt(e.TargetPort),
					})
				}
			}

			if dw.EndpointExposure(e.Exposure) != dw.PublicEndpointExposure {
				continue
			}

			if e.Attributes.GetString(urlRewriteSupportedEndpointAttributeName, nil) == "true" {
				addEndpointToTraefikConfig(componentName, e, wsRouteConfig, cheCluster, routing, endpointStrategy)
			} else {
				ingressExpose(&EndpointInfo{
					order:         order,
					componentName: componentName,
					endpointName:  e.Name,
					port:          int32(e.TargetPort),
					scheme:        determineEndpointScheme(e),
					service:       commonService,
				})
				order = order + 1
			}
		}
	}

	contents, err := yaml.Marshal(wsRouteConfig)
	if err != nil {
		logger.Error(err, "can't serialize traefik config")
	}

	wsConfigMap := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      dwdefaults.GetGatewayWorkspaceConfigMapName(routing.Spec.DevWorkspaceId),
			Namespace: routing.Namespace,
			Labels: map[string]string{
				dwconstants.DevWorkspaceIDLabel:    routing.Spec.DevWorkspaceId,
				constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
			},
		},
		Data: map[string]string{},
	}

	wsConfigMap.Data["workspace.yml"] = string(contents)
	wsConfigMap.Data["traefik.yml"] = fmt.Sprintf(`
entrypoints:
  http:
    address: ":%d"
    forwardedHeaders:
      insecure: true
global:
  checkNewVersion: false
  sendAnonymousUsage: false
providers:
  file:
    filename: "/etc/traefik/workspace.yml"
    watch: false
log:
  level: "INFO"`, wsGatewayPort)

	return wsConfigMap
}

func containPort(service *corev1.Service, port int32) bool {
	for _, p := range service.Spec.Ports {
		if p.Port == port {
			return true
		}
	}
	return false
}

func provisionMainWorkspaceRoute(cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, cmLabels map[string]string, endpointStrategy EndpointStrategy) (*corev1.ConfigMap, error) {
	dwId := routing.Spec.DevWorkspaceId
	dwNamespace := routing.Namespace
	pathPrefix := endpointStrategy.getMainWorkspacePathPrefix()
	priority := 100 + len(pathPrefix)

	cfg := gateway.CreateCommonTraefikConfig(
		dwId,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		priority,
		getServiceURL(wsGatewayPort, dwId, dwNamespace),
		[]string{pathPrefix})

	if cheCluster.IsAccessTokenConfigured() {
		cfg.AddAuthHeaderRewrite(dwId)
	}

	// authorize against kube-rbac-proxy in che-gateway. This will be needed for k8s native auth as well.
	cfg.AddAuth(dwId, "http://127.0.0.1:8089?namespace="+dwNamespace)

	add5XXErrorHandling(cfg, dwId)

	// make '/healthz' path of main endpoints reachable from outside
	routeForHealthzEndpoint(cfg, dwId, routing.Spec.Endpoints, priority+1, endpointStrategy)

	if contents, err := yaml.Marshal(cfg); err != nil {
		return nil, err
	} else {
		return &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      dwdefaults.GetGatewayWorkspaceConfigMapName(dwId),
				Namespace: cheCluster.Namespace,
				Labels:    cmLabels,
				Annotations: map[string]string{
					dwdefaults.ConfigAnnotationDevWorkspaceRoutingName:      routing.Name,
					dwdefaults.ConfigAnnotationDevWorkspaceRoutingNamespace: routing.Namespace,
				},
			},
			Data: map[string]string{dwId + ".yml": string(contents)},
		}, nil
	}
}

// add5XXErrorHandling adds traefik middlewares to the traefik config such that
// when a connection cannot be established with the workspace service (causing a 5XX error code), traefik
// routes the request to the dashboard service instead.
func add5XXErrorHandling(cfg *gateway.TraefikConfig, dwId string) {
	// revalidate cache to prevent case where redirect to dashboard after trying to restart an idled workspace
	noCacheHeader := map[string]string{"cache-control": "no-store, max-age=0"}
	cfg.AddResponseHeaders(dwId, noCacheHeader)

	// dashboard service name must match Traefik dashboard service name
	dashboardServiceName := defaults.GetCheFlavor() + "-dashboard"
	cfg.AddErrors(dwId, "500-599", dashboardServiceName, "/")

	// If a connection cannot be established with the workspace service within the `DialTimeout`, traefik
	// will retry the connection with an exponential backoff
	cfg.HTTP.ServersTransports = map[string]*gateway.TraefikConfigServersTransport{}

	cfg.HTTP.ServersTransports[dwId] = &gateway.TraefikConfigServersTransport{
		ForwardingTimeouts: &gateway.TraefikConfigForwardingTimeouts{
			DialTimeout: "2500ms",
		},
	}
	cfg.AddRetry(dwId, 2, "500ms")
	cfg.HTTP.Services[dwId].LoadBalancer.ServersTransport = dwId
}

// makes '/healthz' path of main endpoints reachable from the outside
func routeForHealthzEndpoint(cfg *gateway.TraefikConfig, dwId string, endpoints map[string]dwo.EndpointList, priority int, endpointStrategy EndpointStrategy) {
	for componentName, endpoints := range endpoints {
		for _, e := range endpoints {
			if e.Attributes.GetString(string(dwo.TypeEndpointAttribute), nil) == string(dwo.MainEndpointType) {
				middlewares := []string{dwId + gateway.StripPrefixMiddlewareSuffix}
				if infrastructure.IsOpenShift() {
					middlewares = append(middlewares, dwId+gateway.HeaderRewriteMiddlewareSuffix)
				}
				routeName, endpointPath := endpointStrategy.getEndpointPath(&e, componentName)
				routerName := fmt.Sprintf("%s-%s-healthz", dwId, routeName)
				cfg.HTTP.Routers[routerName] = &gateway.TraefikConfigRouter{
					Rule:        fmt.Sprintf("Path(`%s/healthz`)", endpointStrategy.getEndpointPathPrefix(endpointPath)),
					Service:     dwId,
					Middlewares: middlewares,
					Priority:    priority,
				}
			}
		}
	}
}

func addEndpointToTraefikConfig(componentName string, e dwo.Endpoint, cfg *gateway.TraefikConfig, cheCluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting, endpointStrategy EndpointStrategy) {
	routeName, prefix := endpointStrategy.getEndpointPath(&e, componentName)
	rulePrefix := fmt.Sprintf("PathPrefix(`%s`)", prefix)
	priority := 100 + len(prefix)

	// skip if exact same route is already exposed
	for _, r := range cfg.HTTP.Routers {
		if r.Rule == rulePrefix {
			return
		}
	}

	name := fmt.Sprintf("%s-%s-%s", routing.Spec.DevWorkspaceId, componentName, routeName)
	cfg.AddComponent(
		name,
		rulePrefix,
		priority,
		fmt.Sprintf("http://127.0.0.1:%d", e.TargetPort),
		[]string{prefix})
	cfg.AddAuth(name, fmt.Sprintf("http://%s.%s:8089?namespace=%s", gateway.GatewayServiceName, cheCluster.Namespace, routing.Namespace))

	// we need to disable auth for '/healthz' path in main endpoint, for now only on OpenShift
	if e.Attributes.GetString(string(dwo.TypeEndpointAttribute), nil) == string(dwo.MainEndpointType) {
		healthzName := name + "-healthz"
		healthzPath := prefix + "/healthz"
		cfg.AddComponent(
			healthzName,
			fmt.Sprintf("Path(`%s`)", healthzPath),
			priority+1,
			fmt.Sprintf("http://127.0.0.1:%d", e.TargetPort),
			[]string{prefix})
	}
}

func findServiceForPort(port int32, objs *solvers.RoutingObjects) *corev1.Service {
	for i := range objs.Services {
		svc := &objs.Services[i]
		for j := range svc.Spec.Ports {
			if svc.Spec.Ports[j].Port == port {
				return svc
			}
		}
	}

	return nil
}

func findIngressForEndpoint(componentName string, endpoint dwo.Endpoint, objs *solvers.RoutingObjects) *networkingv1.Ingress {
	for i := range objs.Ingresses {
		ingress := &objs.Ingresses[i]

		if ingress.Annotations[dwdefaults.ConfigAnnotationComponentName] != componentName ||
			ingress.Annotations[dwdefaults.ConfigAnnotationEndpointName] != endpoint.Name {
			continue
		}

		for r := range ingress.Spec.Rules {
			rule := ingress.Spec.Rules[r]
			for p := range rule.HTTP.Paths {
				path := rule.HTTP.Paths[p]
				if path.Backend.Service.Port.Number == int32(endpoint.TargetPort) {
					return ingress
				}
			}
		}
	}

	return nil
}

func findRouteForEndpoint(componentName string, endpoint dwo.Endpoint, objs *solvers.RoutingObjects, dwId string) *routeV1.Route {
	service := findServiceForPort(int32(endpoint.TargetPort), objs)
	if service == nil {
		service = getCommonService(objs, dwId)
	}
	if service == nil {
		return nil
	}

	for r := range objs.Routes {
		route := &objs.Routes[r]
		if route.Annotations[dwdefaults.ConfigAnnotationComponentName] == componentName &&
			route.Annotations[dwdefaults.ConfigAnnotationEndpointName] == endpoint.Name &&
			route.Spec.To.Kind == "Service" &&
			route.Spec.To.Name == service.Name &&
			route.Spec.Port.TargetPort.IntValue() == endpoint.TargetPort {
			return route
		}
	}

	return nil
}

func (c *CheRoutingSolver) cheRoutingFinalize(cheManager *chev2.CheCluster, routing *dwo.DevWorkspaceRouting) error {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", dwconstants.DevWorkspaceIDLabel, routing.Spec.DevWorkspaceId))
	if err != nil {
		return err
	}

	// delete configs from che namespace
	if deleteErr := c.deleteConfigs(&client.ListOptions{
		Namespace:     cheManager.Namespace,
		LabelSelector: selector,
	}); deleteErr != nil {
		return deleteErr
	}

	// delete configs from workspace namespace
	if deleteErr := c.deleteConfigs(&client.ListOptions{
		Namespace:     routing.Namespace,
		LabelSelector: selector,
	}); deleteErr != nil {
		return deleteErr
	}

	return nil
}

func (c *CheRoutingSolver) deleteConfigs(listOpts *client.ListOptions) error {
	configs := &corev1.ConfigMapList{}
	err := c.client.List(context.TODO(), configs, listOpts)
	if err != nil {
		return err
	}

	for _, cm := range configs.Items {
		err = c.client.Delete(context.TODO(), &cm)
		if err != nil {
			return err
		}
	}
	return nil
}

func getServiceURL(port int32, workspaceID string, workspaceNamespace string) string {
	// the default .cluster.local suffix of the internal domain names seems to be configurable, so let's just
	// not use it so we don't have to know about it...
	return fmt.Sprintf("http://%s.%s.svc:%d", common.ServiceName(workspaceID), workspaceNamespace, port)
}

func getPublicURLPrefixForEndpoint(componentName string, endpoint dwo.Endpoint, endpointStrategy EndpointStrategy) string {
	endpointName := ""
	if endpoint.Attributes.GetString(uniqueEndpointAttributeName, nil) == "true" {
		endpointName = endpoint.Name
	}
	return endpointStrategy.getPublicURLPrefix(int32(endpoint.TargetPort), endpointName, componentName)
}

func determineEndpointScheme(e dwo.Endpoint) string {
	var scheme string
	if e.Protocol == "" {
		scheme = "http"
	} else {
		scheme = string(e.Protocol)
	}

	upgradeToSecure := e.Secure

	// gateway is always on HTTPS, so if the endpoint is served through the gateway, we need to use the TLS'd variant.
	if e.Attributes.GetString(urlRewriteSupportedEndpointAttributeName, nil) == "true" {
		upgradeToSecure = true
	}

	if upgradeToSecure {
		scheme = secureScheme(scheme)
	}

	return scheme
}
