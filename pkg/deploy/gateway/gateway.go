//
// Copyright (c) 2020-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package gateway

import (
	"context"
	"fmt"
	"strconv"

	"github.com/eclipse/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// GatewayServiceName is the name of the service which through which the gateway can be accessed
	GatewayServiceName = "che-gateway"

	gatewayServerConfigName    = "che-gateway-route-server"
	gatewayConfigComponentName = "che-gateway-config"
)

var (
	serviceAccountDiffOpts = cmpopts.IgnoreFields(corev1.ServiceAccount{}, "TypeMeta", "ObjectMeta", "Secrets", "ImagePullSecrets")
	roleDiffOpts           = cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta", "ObjectMeta")
	roleBindingDiffOpts    = cmpopts.IgnoreFields(rbac.RoleBinding{}, "TypeMeta", "ObjectMeta")
	serviceDiffOpts        = cmp.Options{
		cmpopts.IgnoreFields(corev1.Service{}, "TypeMeta", "ObjectMeta", "Status"),
		cmpopts.IgnoreFields(corev1.ServiceSpec{}, "ClusterIP"),
	}
	configMapDiffOpts = cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta")
)

// SyncGatewayToCluster installs or deletes the gateway based on the custom resource configuration
func SyncGatewayToCluster(deployContext *deploy.DeployContext) error {
	if deployContext.CheCluster.Spec.Server.ServerExposureStrategy == "single-host" &&
		(deploy.GetSingleHostExposureType(deployContext.CheCluster) == "gateway") {
		return syncAll(deployContext)
	}

	return deleteAll(deployContext)
}

func syncAll(deployContext *deploy.DeployContext) error {
	instance := deployContext.CheCluster
	sa := getGatewayServiceAccountSpec(instance)
	if err := sync(deployContext, &sa, serviceAccountDiffOpts); err != nil {
		return err
	}

	role := getGatewayRoleSpec(instance)
	if err := sync(deployContext, &role, roleDiffOpts); err != nil {
		return err
	}

	roleBinding := getGatewayRoleBindingSpec(instance)
	if err := sync(deployContext, &roleBinding, roleBindingDiffOpts); err != nil {
		return err
	}

	traefikConfig := getGatewayTraefikConfigSpec(instance)
	if err := sync(deployContext, &traefikConfig, configMapDiffOpts); err != nil {
		return err
	}

	depl := getGatewayDeploymentSpec(instance)
	if err := sync(deployContext, &depl, deploy.DeploymentDiffOpts); err != nil {
		return err
	}

	service := getGatewayServiceSpec(instance)
	if err := sync(deployContext, &service, serviceDiffOpts); err != nil {
		return err
	}

	serverConfig := getGatewayServerConfigSpec(deployContext)
	if err := sync(deployContext, &serverConfig, configMapDiffOpts); err != nil {
		return err
	}

	return nil
}

func deleteAll(deployContext *deploy.DeployContext) error {
	instance := deployContext.CheCluster
	clusterAPI := deployContext.ClusterAPI

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &deployment); err != nil {
		return err
	}

	serverConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayServerConfigName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &serverConfig); err != nil {
		return err
	}

	traefikConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config",
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &traefikConfig); err == nil {
		return err
	}

	roleBinding := rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &roleBinding); err == nil {
		return err
	}

	role := rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &role); err == nil {
		return err
	}

	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &sa); err == nil {
		return err
	}

	return nil
}

// sync syncs the blueprint to the cluster in a generic (as much as Go allows) manner.
func sync(deployContext *deploy.DeployContext, blueprint metav1.Object, diffOpts cmp.Option) error {
	clusterAPI := deployContext.ClusterAPI

	blueprintObject, ok := blueprint.(runtime.Object)
	if !ok {
		return fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", blueprint)
	}

	key := client.ObjectKey{Name: blueprint.GetName(), Namespace: blueprint.GetNamespace()}

	actual := blueprintObject.DeepCopyObject()

	if getErr := deployContext.ClusterAPI.Client.Get(context.TODO(), key, actual); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return getErr
		}
		actual = nil
	}

	kind := blueprintObject.GetObjectKind().GroupVersionKind().Kind

	if actual == nil {
		logrus.Infof("Creating a new object: %s, name %s", kind, blueprint.GetName())
		obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
		if err != nil {
			return err
		}

		err = clusterAPI.Client.Create(context.TODO(), obj)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}

			// ok, we got an already-exists error. So let's try to load the object into "actual".
			// if we fail this retry for whatever reason, just give up rather than retrying this in a loop...
			// the reconciliation loop will lead us here again in the next round.
			if getErr := deployContext.ClusterAPI.Client.Get(context.TODO(), key, actual); getErr != nil {
				return getErr
			}
		}
	}

	if actual != nil {
		actualMeta := actual.(metav1.Object)

		diff := cmp.Diff(actual, blueprint, diffOpts)
		if len(diff) > 0 {
			logrus.Infof("Updating existing object: %s, name: %s", kind, actualMeta.GetName())
			fmt.Printf("Difference:\n%s", diff)

			if isUpdateUsingDeleteCreate(actual.GetObjectKind().GroupVersionKind().Kind) {
				err := clusterAPI.Client.Delete(context.TODO(), actual)
				if err != nil {
					return err
				}

				obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
				if err != nil {
					return err
				}

				err = clusterAPI.Client.Create(context.TODO(), obj)
				if err != nil {
					return err
				}
			} else {
				obj, err := setOwnerReferenceAndConvertToRuntime(deployContext, blueprint)
				if err != nil {
					return err
				}

				err = clusterAPI.Client.Update(context.TODO(), obj)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func isUpdateUsingDeleteCreate(kind string) bool {
	return "Service" == kind || "Ingress" == kind || "Route" == kind
}

func setOwnerReferenceAndConvertToRuntime(deployContext *deploy.DeployContext, obj metav1.Object) (runtime.Object, error) {
	err := controllerutil.SetControllerReference(deployContext.CheCluster, obj, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	robj, ok := obj.(runtime.Object)
	if !ok {
		return nil, fmt.Errorf("object %T is not a runtime.Object. Cannot sync it", obj)
	}

	return robj, nil
}

func delete(clusterAPI deploy.ClusterAPI, obj metav1.Object) error {
	key := client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	ro := obj.(runtime.Object)
	if getErr := clusterAPI.Client.Get(context.TODO(), key, ro); getErr == nil {
		if err := clusterAPI.Client.Delete(context.TODO(), ro); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// GetGatewayRouteConfig creates a config map with traefik configuration for a single new route.
// `serviceName` is an arbitrary name identifying the configuration. This should be unique within operator. Che server only creates
// new configuration for workspaces, so the name should not resemble any of the names created by the Che server.
func GetGatewayRouteConfig(deployContext *deploy.DeployContext, serviceName string, pathPrefix string, priority int, internalUrl string, stripPrefix bool) corev1.ConfigMap {
	pathRewrite := pathPrefix != "/" && stripPrefix

	data := `---
http:
  routers:
    ` + serviceName + `:
      rule: "PathPrefix(` + "`" + pathPrefix + "`" + `)"
      service: ` + serviceName + `
      priority: ` + strconv.Itoa(priority)

	if pathRewrite {
		data += `
      middlewares:
      - "` + serviceName + `"`
	}

	data += `
  services:
    ` + serviceName + `:
      loadBalancer:
        servers:
        - url: '` + internalUrl + `'`

	if pathRewrite {
		data += `
  middlewares:
    ` + serviceName + `:
      stripPrefix:
        prefixes:
        - "` + pathPrefix + `"`
	}

	ret := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: deployContext.CheCluster.Namespace,
			Labels: util.MergeMaps(
				deploy.GetLabels(deployContext.CheCluster, gatewayConfigComponentName),
				util.GetMapValue(deployContext.CheCluster.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels)),
		},
		Data: map[string]string{
			serviceName + ".yml": data,
		},
	}

	controllerutil.SetControllerReference(deployContext.CheCluster, &ret, deployContext.ClusterAPI.Scheme)

	return ret
}

func DeleteGatewayRouteConfig(serviceName string, deployContext *deploy.DeployContext) error {
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: deployContext.CheCluster.Namespace,
		},
	}

	return delete(deployContext.ClusterAPI, obj)
}

// below functions declare the desired states of the various objects required for the gateway

func getGatewayServerConfigSpec(deployContext *deploy.DeployContext) corev1.ConfigMap {
	return GetGatewayRouteConfig(deployContext, gatewayServerConfigName, "/", 1, "http://"+deploy.CheServiceName+":8080", false)
}

func getGatewayServiceAccountSpec(instance *orgv1.CheCluster) corev1.ServiceAccount {
	return corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
	}
}

func getGatewayRoleSpec(instance *orgv1.CheCluster) rbac.Role {
	return rbac.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbac.SchemeGroupVersion.String(),
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Rules: []rbac.PolicyRule{
			{
				Verbs:     []string{"watch", "get", "list"},
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
			},
		},
	}
}

func getGatewayRoleBindingSpec(instance *orgv1.CheCluster) rbac.RoleBinding {
	return rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbac.SchemeGroupVersion.String(),
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     GatewayServiceName,
		},
		Subjects: []rbac.Subject{
			{
				Kind: "ServiceAccount",
				Name: GatewayServiceName,
			},
		},
	}
}

func getGatewayTraefikConfigSpec(instance *orgv1.CheCluster) corev1.ConfigMap {
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config",
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Data: map[string]string{
			"traefik.yml": `
entrypoints:
  http:
    address: ":8080"
    forwardedHeaders:
      insecure: true
  https:
    address: ":8443"
    forwardedHeaders:
      insecure: true
global:
  checkNewVersion: false
  sendAnonymousUsage: false
providers:
  file:
    directory: "/dynamic-config"
    watch: true
log:
  level: "INFO"`,
		},
	}
}

func getGatewayDeploymentSpec(instance *orgv1.CheCluster) appsv1.Deployment {
	gatewayImage := util.GetValue(instance.Spec.Server.SingleHostGatewayImage, deploy.DefaultSingleHostGatewayImage(instance))
	sidecarImage := util.GetValue(instance.Spec.Server.SingleHostGatewayConfigSidecarImage, deploy.DefaultSingleHostGatewayConfigSidecarImage(instance))
	configLabelsMap := util.GetMapValue(instance.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels)
	terminationGracePeriodSeconds := int64(10)

	configLabels := labels.FormatLabels(configLabelsMap)

	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: deploy.GetLabels(instance, GatewayServiceName),
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deploy.GetLabels(instance, GatewayServiceName),
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					ServiceAccountName:            GatewayServiceName,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:            "gateway",
							Image:           gatewayImage,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "static-config",
									MountPath: "/etc/traefik",
								},
								{
									Name:      "dynamic-config",
									MountPath: "/dynamic-config",
								},
							},
						},
						{
							Name:            "configbump",
							Image:           sidecarImage,
							ImagePullPolicy: corev1.PullAlways,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "dynamic-config",
									MountPath: "/dynamic-config",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "CONFIG_BUMP_DIR",
									Value: "/dynamic-config",
								},
								{
									Name:  "CONFIG_BUMP_LABELS",
									Value: configLabels,
								},
								{
									Name: "CONFIG_BUMP_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "static-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "che-gateway-config",
									},
								},
							},
						},
						{
							Name: "dynamic-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func getGatewayServiceSpec(instance *orgv1.CheCluster) corev1.Service {
	return corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Spec: corev1.ServiceSpec{
			Selector:        deploy.GetLabels(instance, GatewayServiceName),
			SessionAffinity: corev1.ServiceAffinityNone,
			Type:            corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "gateway-http",
					Port:       8080,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8080),
				},
				{
					Name:       "gateway-https",
					Port:       8443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8443),
				},
			},
		},
	}
}
