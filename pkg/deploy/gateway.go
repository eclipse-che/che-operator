package deploy

import (
	"context"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SyncGatewayToCluster installs or deletes the gateway based on the custom resource configuration
func SyncGatewayToCluster(instance *orgv1.CheCluster, clusterAPI ClusterAPI) error {
	if instance.Spec.Server.ServerExposureStrategy == "single-host" &&
		(util.IsOpenShift || instance.Spec.Server.SingleHostWorkspaceExposureType == "gateway") {

		return createGateway(instance, clusterAPI)
	} else {
		return deleteGateway(instance, clusterAPI)
	}
}

func createGateway(instance *orgv1.CheCluster, clusterAPI ClusterAPI) error {
	gatewayImage := util.GetValue(instance.Spec.Server.SingleHostGatewayImage, DefaultSingleHostGatewayImage)
	sidecarImage := util.GetValue(instance.Spec.Server.SingleHostGatewayConfigSidecarImage, DefaultSingleHostGatewayConfigSidecarImage)

	// Create the SA for the gateway with the minimal permissions
	sa := &corev1.ServiceAccount{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, sa); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return getErr
		}

		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "che-gateway",
			},
		}

		err := controllerutil.SetControllerReference(instance, sa, clusterAPI.Scheme)
		if err != nil {
			return err
		}

		if err := clusterAPI.Client.Create(context.TODO(), sa); err != nil {
			return err
		}
	}

	role := &rbac.Role{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, role); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return getErr
		}

		role = &rbac.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "che-gateway",
			},
			Rules: []rbac.PolicyRule{
				{
					Verbs:     []string{"watch", "get", "list"},
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
				},
			},
		}

		err := controllerutil.SetControllerReference(instance, role, clusterAPI.Scheme)
		if err != nil {
			return err
		}

		if err := clusterAPI.Client.Create(context.TODO(), role); err != nil {
			return err
		}
	}

	roleBinding := &rbac.RoleBinding{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, roleBinding); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return getErr
		}

		roleBinding = &rbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "che-gateway",
			},
			RoleRef: rbac.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "che-gateway",
			},
			Subjects: []rbac.Subject{
				{
					Kind: "Role",
					Name: "che-gateway",
				},
			},
		}

		err := controllerutil.SetControllerReference(instance, roleBinding, clusterAPI.Scheme)
		if err != nil {
			return err
		}

		if err := clusterAPI.Client.Create(context.TODO(), roleBinding); err != nil {
			return err
		}
	}

	traefikConfig := &corev1.ConfigMap{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway-config"}, traefikConfig); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return getErr
		}

		traefikConfig = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "che-gateway-config",
			},
			Data: map[string]string{
				"traefik.yml": `global:
				                  checkNewVersion: false
				                  sendAnonymousUsage: false
				                entrypoints:
				                  http:
					                address: ":8080"
				                  https:
					                address: ":8443"   
				                providers:
				                  file:
					              directory: "/dynamic-config"
					              watch: true
				               `,
			},
		}

		err := controllerutil.SetControllerReference(instance, traefikConfig, clusterAPI.Scheme)
		if err != nil {
			return err
		}

		if err := clusterAPI.Client.Create(context.TODO(), traefikConfig); err != nil {
			return err
		}
	}

	depl := &appsv1.Deployment{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, depl); getErr != nil {
		if statusErr, ok := getErr.(*errors.StatusError); !ok || statusErr.Status().Reason != metav1.StatusReasonNotFound {
			return getErr
		}

		depl = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "che-gateway",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "gateway",
								Image: gatewayImage,
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
								Name:  "configbump",
								Image: sidecarImage,
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
										Value: "app=che,role=gateway-config",
									},
									{
										Name: "CONFIG_BUMP_NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{
												FieldPath: "metadata.namespace",
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

		err := controllerutil.SetControllerReference(instance, depl, clusterAPI.Scheme)
		if err != nil {
			return err
		}

		return clusterAPI.Client.Create(context.TODO(), depl)
	}

	return nil
}

func deleteGateway(instance *orgv1.CheCluster, clusterAPI ClusterAPI) error {
	deployment := &appsv1.Deployment{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, deployment); getErr == nil {
		if err := clusterAPI.Client.Delete(context.TODO(), deployment); err != nil {
			return err
		}
	}

	configMap := &corev1.ConfigMap{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway-config"}, configMap); getErr == nil {
		if err := clusterAPI.Client.Delete(context.TODO(), configMap); err != nil {
			return err
		}
	}

	roleBinding := &rbac.RoleBinding{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, roleBinding); getErr == nil {
		if err := clusterAPI.Client.Delete(context.TODO(), roleBinding); err != nil {
			return err
		}
	}

	role := &rbac.Role{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, role); getErr == nil {
		if err := clusterAPI.Client.Delete(context.TODO(), role); err != nil {
			return err
		}
	}

	sa := &corev1.ServiceAccount{}
	if getErr := clusterAPI.Client.Get(context.TODO(), client.ObjectKey{Name: "che-gateway"}, sa); getErr == nil {
		if err := clusterAPI.Client.Delete(context.TODO(), sa); err != nil {
			return err
		}
	}

	return nil
}
