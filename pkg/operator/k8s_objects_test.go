package operator

import (
	"github.com/sirupsen/logrus"
	"os"
	"testing"
)

func TestNewCheDeployment(t *testing.T) {

	// create a fake Che deployment
	fakeImage := "fake-image"
	deployment, err := fakeK8s.clientset.AppsV1().Deployments("eclipse-che").Create(newCheDeployment(fakeImage))
	if err != nil {
		panic(err)
	}
	logrus.Infof("Deployment %s created", deployment.Name)

	if deployment.Spec.Template.Spec.Containers[0].Image != fakeImage {
		logrus.Fatalf("Expected %s, but got %s", fakeImage, deployment.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestNewCheConfigMap(t *testing.T) {
	// set current infra to k8s
	err := os.Setenv("INFRA", "kubernetes")
	if err != nil {
		logrus.Fatal(err)
	}

	// set fake ingress domain env and check if configmap grabs its value
	fakeDomain := "fake-domain"
	fakeDomainEnv := "CHE_INFRA_KUBERNETES_INGRESS_DOMAIN"
	err = os.Setenv(fakeDomainEnv, fakeDomain)
	if err != nil {
		logrus.Fatal(err)
	}

	// create a configmap
	cm, err := fakeK8s.clientset.CoreV1().ConfigMaps("eclipse-che").Create(newCheConfigMap(cheHost, keycloakURL))
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("ConfigMap %s successfully created", cm.Name)

	// check env and its value
	cmEnv := cm.Data
	if val, ok := cmEnv["CHE_INFRA_KUBERNETES_INGRESS_DOMAIN"]; ok {
		logrus.Infof("Env found. CHE_INFRA_KUBERNETES_INGRESS_DOMAIN=%s", val)
		if val != fakeDomain {
			logrus.Fatalf("Expected %s but got %s", fakeDomain, val)
		}
	} else {
		logrus.Fatalf("Env %s not found", fakeDomain)
	}
}

func TestNewPostgresDeployment(t *testing.T) {

	// create a fake Postgres deployment
	deployment, err := fakeK8s.clientset.AppsV1().Deployments("eclipse-che").Create(newPostgresDeployment())
	if err != nil {
		panic(err)
	}
	logrus.Infof("Deployment %s created", deployment.Name)
	password := deployment.Spec.Template.Spec.Containers[0].Env[1].Value

	// check if password was properly generated and added to container env
	if len(password) != 12 {
		logrus.Fatalf("Expecting password lenght to be 12, but got %v", len(password))
	}
}

func TestNewKeycloakDeployment(t *testing.T) {

	//// create a fake Keycloak deployment
	err := os.Setenv("CHE_SELF__SIGNED__CERT", "mycrt")
	if err != nil {
		logrus.Fatal(err)
	}
	deployment, err := fakeK8s.clientset.AppsV1().Deployments("eclipse-che").Create(newKeycloakDeployment())
	if err != nil {
		panic(err)
	}
	logrus.Infof("Deployment %s created", deployment.Name)

	// check if setting self-signed-certificate cert updates env for the container properly
	selfSignedEnv := deployment.Spec.Template.Spec.Containers[0].Env
	var foundEnv bool
	ssoEnv := "SSO_TRUSTSTORE"
	for i := range selfSignedEnv {
		env := selfSignedEnv[i].Name
		if env == ssoEnv {
			logrus.Infof("%s env found: %s", ssoEnv, env)
			foundEnv = true
		}
	}
	if !foundEnv {
		logrus.Fatalf("No %s env found", ssoEnv)
	}
}
