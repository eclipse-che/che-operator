package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"time"
)

func newCheDeployment(cheImageRepo string, cheImageTag string) *appsv1.Deployment {
	cheLabels := map[string]string{"app": "che"}
	optionalEnv := true
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che",
			Namespace: namespace,
			Labels:    cheLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: cheLabels},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyType("Recreate"),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: cheLabels,
				},
				Spec: corev1.PodSpec{
					// testing https on k8s
					HostAliases: hostAliases,
					Volumes: []corev1.Volume{
						{
							Name: "che-data-volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "che-data-volume",
								},
							},
						},
					},
					ServiceAccountName: "che",
					Containers: []corev1.Container{
						{
							Name:  "che",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Image: cheImageRepo + ":" + cheImageTag,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
								{
									Name:          "http-debug",
									ContainerPort: 8000,
									Protocol:      "TCP",
								},
								{
									Name:          "jgroups-ping",
									ContainerPort: 8888,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "che-data-volume",
									MountPath: "/data",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/system/state",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(8080),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 25,
								FailureThreshold:    5,
								TimeoutSeconds:      5,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/api/system/state",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(8080),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 50,
								FailureThreshold:    3,
								TimeoutSeconds:      3,
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: "che"},
									},
								},
							},
							Env: []corev1.EnvVar{
								// todo Add OPENSHIFT_SELF_SIGNED_CERT form secret
								{
									Name: "OPENSHIFT_KUBE_PING_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace"}},
								},
								{
									Name: "OPENSHIFT_IDENTITY_PROVIDER_CERTIFICATE",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "ca.crt",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "self-signed-cert",
											},
											Optional: &optionalEnv,
										},
									},
								},
							}},
					},
				},
			},
		},
	}
}

// CreateCheDeployment creates a deployment with che ConfigMap in env
func CreateCheDeployment(cheImageRepo string, cheImageTag string) (*appsv1.Deployment, error) {
	deployment := newCheDeployment(cheImageRepo, cheImageTag)
	if err := sdk.Create(deployment); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create Che deployment : %v", err)
		return nil, err
	}
	// wait until deployment is scaled to 1 replica to proceed with other deployments
	util.WaitForSuccessfulDeployment(deployment, "Che", 40)

	logrus.Info("Che is available at: " + protocol + "://" + cheHost)
	deploymentTime := time.Since(StartTime)
	logrus.Info("Deployment took ", deploymentTime)


	return deployment, nil
}
