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

package solver

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/util"

	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/sync"
	"github.com/eclipse-che/che-operator/pkg/deploy"
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

func (c *CheRoutingSolver) cheSpecObjects(cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting, workspaceMeta solvers.DevWorkspaceMetadata) (solvers.RoutingObjects, error) {
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

func (c *CheRoutingSolver) provisionServices(objs *solvers.RoutingObjects, cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting, workspaceMeta solvers.DevWorkspaceMetadata) error {
	objs.Services = solvers.GetDiscoverableServicesForEndpoints(routing.Spec.Endpoints, workspaceMeta)

	commonService := &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name:      common.ServiceName(routing.Spec.DevWorkspaceId),
			Namespace: routing.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:   routing.Spec.DevWorkspaceId,
				deploy.KubernetesPartOfLabelKey: deploy.CheEclipseOrg,
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
	annos[defaults.ConfigAnnotationCheManagerName] = cheCluster.Name
	annos[defaults.ConfigAnnotationCheManagerNamespace] = cheCluster.Namespace

	additionalLabels := defaults.GetLabelsForComponent(cheCluster, "exposure")

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

func (c *CheRoutingSolver) provisionRouting(objs *solvers.RoutingObjects, cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting, workspaceMeta solvers.DevWorkspaceMetadata) error {
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

func (c *CheRoutingSolver) provisionPodAdditions(objs *solvers.RoutingObjects, cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting) error {
	objs.PodAdditions = &dwo.PodAdditions{
		Containers: []corev1.Container{},
		Volumes:    []corev1.Volume{},
	}
	objs.PodAdditions.Containers = append(objs.PodAdditions.Containers, corev1.Container{
		Name:            wsGatewayName,
		Image:           cheCluster.Spec.Gateway.Image,
		ImagePullPolicy: corev1.PullPolicy(deploy.DefaultPullPolicyFromDockerImage(cheCluster.Spec.Gateway.Image)),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      wsGatewayName,
				MountPath: "/etc/traefik",
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("256Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.5"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("64Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.05"),
			},
		},
	})

	// Even though DefaultMode is optional in Kubernetes, DevWorkspace Controller needs it to be explicitly defined.
	// 420 = 0644 = '-rw-r--r--'
	defaultMode := int32(420)
	objs.PodAdditions.Volumes = append(objs.PodAdditions.Volumes, corev1.Volume{
		Name: wsGatewayName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: defaults.GetGatewayWorkspaceConfigMapName(routing.Spec.DevWorkspaceId),
				},
				DefaultMode: &defaultMode,
			},
		},
	})

	return nil
}

func (c *CheRoutingSolver) cheExposedEndpoints(cheCluster *v2alpha1.CheCluster, workspaceID string, componentEndpoints map[string]dwo.EndpointList, routingObj solvers.RoutingObjects) (exposedEndpoints map[string]dwo.ExposedEndpointList, ready bool, err error) {
	if cheCluster.Status.GatewayPhase == v2alpha1.GatewayPhaseInitializing {
		return nil, false, nil
	}

	exposedEndpoints = map[string]dwo.ExposedEndpointList{}

	gatewayHost := cheCluster.Status.GatewayHost

	for component, endpoints := range componentEndpoints {
		for _, endpoint := range endpoints {
			if endpoint.Exposure != dw.PublicEndpointExposure {
				continue
			}

			scheme := determineEndpointScheme(endpoint)

			if !isExposableScheme(scheme) {
				// we cannot expose non-http endpoints publicly, because ingresses/routes only support http(s)
				continue
			}

			// try to find the endpoint in the ingresses/routes first. If it is there, it is exposed on a subdomain
			// otherwise it is exposed through the gateway
			var endpointURL string
			if util.IsOpenShift4 {
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

				publicURLPrefix := getPublicURLPrefixForEndpoint(workspaceID, component, endpoint)
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

func (c *CheRoutingSolver) getGatewayConfigsAndFillRoutingObjects(cheCluster *v2alpha1.CheCluster, workspaceID string, routing *dwo.DevWorkspaceRouting, objs *solvers.RoutingObjects) ([]corev1.ConfigMap, error) {
	restrictedAnno, setRestrictedAnno := routing.Annotations[constants.DevWorkspaceRestrictedAccessAnnotation]

	cmLabels := defaults.AddStandardLabelsForComponent(cheCluster, "gateway-config", defaults.GetGatewayWorkspaceConfigMapLabels(cheCluster))
	cmLabels[constants.DevWorkspaceIDLabel] = workspaceID
	if setRestrictedAnno {
		cmLabels[constants.DevWorkspaceRestrictedAccessAnnotation] = restrictedAnno
	}

	configs := make([]corev1.ConfigMap, 0)

	// first do routing from main che-gateway into workspace service
	if mainWsRouteConfig, err := provisionMainWorkspaceRoute(cheCluster, routing, cmLabels); err != nil {
		return nil, err
	} else {
		configs = append(configs, *mainWsRouteConfig)
	}

	// then expose the endpoints
	if infraExposer, err := c.getInfraSpecificExposer(cheCluster, routing, objs); err != nil {
		return nil, err
	} else {
		if workspaceConfig := exposeAllEndpoints(cheCluster, routing, objs, infraExposer); workspaceConfig != nil {
			configs = append(configs, *workspaceConfig)
		}
	}

	return configs, nil
}

func (c *CheRoutingSolver) getInfraSpecificExposer(cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting, objs *solvers.RoutingObjects) (func(info *EndpointInfo), error) {
	if util.IsOpenShift4 {
		exposer := &RouteExposer{}
		if err := exposer.initFrom(context.TODO(), c.client, cheCluster, routing); err != nil {
			return nil, err
		}
		return func(info *EndpointInfo) {
			route := exposer.getRouteForService(info)
			objs.Routes = append(objs.Routes, route)
		}, nil
	} else {
		exposer := &IngressExposer{}
		if err := exposer.initFrom(context.TODO(), c.client, cheCluster, routing, defaults.GetIngressAnnotations(cheCluster)); err != nil {
			return nil, err
		}
		return func(info *EndpointInfo) {
			ingress := exposer.getIngressForService(info)
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

func exposeAllEndpoints(cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting, objs *solvers.RoutingObjects, ingressExpose func(*EndpointInfo)) *corev1.ConfigMap {
	wsRouteConfig := gateway.CreateEmptyTraefikConfig()

	commonService := getCommonService(objs, routing.Spec.DevWorkspaceId)
	if commonService == nil {
		return nil
	}

	order := 1
	for componentName, endpoints := range routing.Spec.Endpoints {
		for _, e := range endpoints {
			if e.Exposure != dw.PublicEndpointExposure {
				continue
			}

			if e.Attributes.GetString(urlRewriteSupportedEndpointAttributeName, nil) == "true" {
				addEndpointToTraefikConfig(componentName, e, wsRouteConfig, cheCluster, routing)
			} else {
				if !containPort(commonService, int32(e.TargetPort)) {
					commonService.Spec.Ports = append(commonService.Spec.Ports, corev1.ServicePort{
						Name:       common.EndpointName(e.Name),
						Protocol:   corev1.ProtocolTCP,
						Port:       int32(e.TargetPort),
						TargetPort: intstr.FromInt(e.TargetPort),
					})
				}

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
			Name:      defaults.GetGatewayWorkspaceConfigMapName(routing.Spec.DevWorkspaceId),
			Namespace: routing.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel:   routing.Spec.DevWorkspaceId,
				deploy.KubernetesPartOfLabelKey: deploy.CheEclipseOrg,
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

func provisionMainWorkspaceRoute(cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting, cmLabels map[string]string) (*corev1.ConfigMap, error) {
	dwId := routing.Spec.DevWorkspaceId
	dwNamespace := routing.Namespace

	cfg := gateway.CreateCommonTraefikConfig(
		dwId,
		fmt.Sprintf("PathPrefix(`/%s`)", dwId),
		100,
		getServiceURL(wsGatewayPort, dwId, dwNamespace),
		[]string{"/" + dwId})

	if util.IsOpenShift4 {
		// on OpenShift, we need to set authorization header.
		// This MUST come before Auth, because Auth needs Authorization header to be properly set.
		cfg.AddAuthHeaderRewrite(dwId)

		// authorize against kube-rbac-proxy in che-gateway. This will be needed for k8s native auth as well.
		cfg.AddAuth(dwId, "http://127.0.0.1:8089?namespace="+dwNamespace)

		// make '/healthz' path of main endpoints reachable from outside
		routeForHealthzEndpoint(cfg, dwId, routing.Spec.Endpoints)
	}

	if contents, err := yaml.Marshal(cfg); err != nil {
		return nil, err
	} else {
		return &corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      defaults.GetGatewayWorkspaceConfigMapName(dwId),
				Namespace: cheCluster.Namespace,
				Labels:    cmLabels,
				Annotations: map[string]string{
					defaults.ConfigAnnotationDevWorkspaceRoutingName:      routing.Name,
					defaults.ConfigAnnotationDevWorkspaceRoutingNamespace: routing.Namespace,
				},
			},
			Data: map[string]string{dwId + ".yml": string(contents)},
		}, nil
	}
}

// makes '/healthz' path of main endpoints reachable from the outside
func routeForHealthzEndpoint(cfg *gateway.TraefikConfig, dwId string, endpoints map[string]dwo.EndpointList) {
	for componentName, endpoints := range endpoints {
		for _, e := range endpoints {
			if e.Attributes.GetString(string(dwo.TypeEndpointAttribute), nil) == string(dwo.MainEndpointType) {
				middlewares := []string{dwId + gateway.StripPrefixMiddlewareSuffix}
				if util.IsOpenShift4 {
					middlewares = append(middlewares, dwId+gateway.HeaderRewriteMiddlewareSuffix)
				}
				routeName, endpointPath := createEndpointPath(&e, componentName)
				cfg.HTTP.Routers[routeName+"-healthz"] = &gateway.TraefikConfigRouter{
					Rule:        fmt.Sprintf("Path(`/%s%s/healthz`)", dwId, endpointPath),
					Service:     dwId,
					Middlewares: middlewares,
					Priority:    101,
				}
			}
		}
	}
}

func addEndpointToTraefikConfig(componentName string, e dw.Endpoint, cfg *gateway.TraefikConfig, cheCluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting) {
	routeName, prefix := createEndpointPath(&e, componentName)
	rulePrefix := fmt.Sprintf("PathPrefix(`%s`)", prefix)

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
		100,
		fmt.Sprintf("http://127.0.0.1:%d", e.TargetPort),
		[]string{prefix})
	if util.IsOpenShift4 {
		cfg.AddAuth(name, fmt.Sprintf("http://%s.%s:8089?namespace=%s", gateway.GatewayServiceName, cheCluster.Namespace, routing.Namespace))
	}

	// we need to disable auth for '/healthz' path in main endpoint, for now only on OpenShift
	if util.IsOpenShift4 &&
		e.Attributes.GetString(string(dwo.TypeEndpointAttribute), nil) == string(dwo.MainEndpointType) {
		healthzName := name + "-healthz"
		healthzPath := prefix + "/healthz"
		cfg.AddComponent(
			healthzName,
			fmt.Sprintf("Path(`%s`)", healthzPath),
			101,
			fmt.Sprintf("http://127.0.0.1:%d", e.TargetPort),
			[]string{prefix})
	}
}

func createEndpointPath(e *dw.Endpoint, componentName string) (routeName string, path string) {
	if e.Attributes.GetString(uniqueEndpointAttributeName, nil) == "true" {
		// if endpoint is unique, we're exposing on /componentName/<endpoint-name>
		routeName = e.Name
	} else {
		// if endpoint is NOT unique, we're exposing on /componentName/<port-number>
		routeName = strconv.Itoa(e.TargetPort)
	}
	path = fmt.Sprintf("/%s/%s", componentName, routeName)

	return routeName, path
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

func findIngressForEndpoint(componentName string, endpoint dw.Endpoint, objs *solvers.RoutingObjects) *networkingv1.Ingress {
	for i := range objs.Ingresses {
		ingress := &objs.Ingresses[i]

		if ingress.Annotations[defaults.ConfigAnnotationComponentName] != componentName ||
			ingress.Annotations[defaults.ConfigAnnotationEndpointName] != endpoint.Name {
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

func findRouteForEndpoint(componentName string, endpoint dw.Endpoint, objs *solvers.RoutingObjects, dwId string) *routeV1.Route {
	service := findServiceForPort(int32(endpoint.TargetPort), objs)
	if service == nil {
		service = getCommonService(objs, dwId)
	}
	if service == nil {
		return nil
	}

	for r := range objs.Routes {
		route := &objs.Routes[r]
		if route.Annotations[defaults.ConfigAnnotationComponentName] == componentName &&
			route.Annotations[defaults.ConfigAnnotationEndpointName] == endpoint.Name &&
			route.Spec.To.Kind == "Service" &&
			route.Spec.To.Name == service.Name &&
			route.Spec.Port.TargetPort.IntValue() == endpoint.TargetPort {
			return route
		}
	}

	return nil
}

func (c *CheRoutingSolver) cheRoutingFinalize(cheManager *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting) error {
	selector, err := labels.Parse(fmt.Sprintf("%s=%s", constants.DevWorkspaceIDLabel, routing.Spec.DevWorkspaceId))
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

func getPublicURLPrefixForEndpoint(workspaceID string, machineName string, endpoint dw.Endpoint) string {
	endpointName := ""
	if endpoint.Attributes.GetString(uniqueEndpointAttributeName, nil) == "true" {
		endpointName = endpoint.Name
	}

	return getPublicURLPrefix(workspaceID, machineName, int32(endpoint.TargetPort), endpointName)
}

func getPublicURLPrefix(workspaceID string, machineName string, port int32, uniqueEndpointName string) string {
	if uniqueEndpointName == "" {
		return fmt.Sprintf(endpointURLPrefixPattern, workspaceID, machineName, port)
	}
	return fmt.Sprintf(uniqueEndpointURLPrefixPattern, workspaceID, machineName, uniqueEndpointName)
}

func determineEndpointScheme(e dw.Endpoint) string {
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
