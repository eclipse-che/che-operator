package util

import (
	"github.com/operator-framework/operator-sdk/pkg/k8sclient"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	appsv1 "k8s.io/api/apps/v1"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/sirupsen/logrus"
	"io/ioutil"
	"math/rand"
	"os"
	"time"
)

// GetEnvValue looks for env variables in Operator pod to configure Code Ready deployments
// with things like db users, passwords and deployment options in general. Envs are set in
// a ConfigMap at deploy/config.yaml. Find more details on deployment options in README.md

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func GetEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	if value == "true" {
		return true
	}
	return false
}

func GeneratePasswd(stringLength int) (passwd string) {
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789")
	length := stringLength
	buf := make([]rune, length)
	for i := range buf {
		buf[i] = chars[rand.Intn(len(chars))]
	}
	passwd = string(buf)
	return passwd
}

func GetInfra() (infra string) {
	kubeClient := k8sclient.GetKubeClient()
	serverGroups, _ := kubeClient.Discovery().ServerGroups()
	apiGroups := serverGroups.Groups

	for i := range apiGroups {
		name := apiGroups[i].Name
		if name == "route.openshift.io" {
			infra = "openshift"
		}
	}
	if infra == "" {
		infra = "kubernetes"
	}
	return infra
}

func GetNamespace() (currentNamespace string) {

	namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		panic(err)
	}
	currentNamespace = string(namespace)
	return currentNamespace
}

func WaitForSuccessfulDeployment(deployment *appsv1.Deployment, name string, checkAttempts int) {
	attempts := 1
	replicas := deployment.Status.AvailableReplicas
	if replicas < 1 {
		for {
			attempts++
			if attempts > checkAttempts {
				logrus.Error("Failed to verify a successful " + name + " deployment. Operator is exiting")
				logrus.Error("Get deployment logs: kubectl logs deployment/" + deployment.Name + " -n=" + deployment.Namespace)
				logrus.Error("Get k8s events: kubectl get events " +
					"--field-selector " +
					"involvedObject.name=$(kubectl get pods -l=app=postgres -n=" + deployment.Namespace +
					" -o=jsonpath='{.items[0].metadata.name}') -n=" + deployment.Namespace)
				os.Exit(1)
			}
			err := sdk.Get(deployment)
			if err != nil {
				logrus.Errorf("Failed to get" + name + " deployment", err)
			}
			replicas := deployment.Status.AvailableReplicas
			attemptsLeft := checkAttempts - attempts
			timeout := attemptsLeft * 5
			logrus.Infof("Waiting for " + name + " deployment to complete. " +
				"Current replicas number: %d. " +
				"Retries left: %d. " +
				"Timeout in: %d seconds", replicas, attemptsLeft, timeout)
			if replicas == 1 {
				logrus.Info("Deployment " + name + " successfully scaled to 1")
				break
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func WaitForSuccessfulJobExecution(job *batchv1.Job, name string, checkAttempts int) {
	attempts := 1
	successfulPods := job.Status.Succeeded
	if successfulPods < 1 {
		for {
			attempts++
			if attempts > checkAttempts {
				logrus.Error("Failed to verify a successful " + name + " job execution. Operator is exiting")
				logrus.Error("Check job logs: kubectl logs job/" + job.Name + " -n=" + job.Namespace)
				logrus.Error("Get k8s events: kubectl get events " +
					"--field-selector involvedObject.name=$(kubectl get pods -l=app=" + job.Name + " -n=" + job.Namespace +
					" -o=jsonpath='{.items[0].metadata.name}') -n=" + job.Namespace)
				os.Exit(1)
			}
			err := sdk.Get(job)
			if err != nil {
				logrus.Errorf("Failed to get" + name + " job", err)
			}
			successfulPods := job.Status.Succeeded
			attemptsLeft := checkAttempts - attempts
			timeout := attemptsLeft * 5
			logrus.Infof("Waiting for " + name + " job pod number to reach 1. " +
				"Current number: %d. " +
				"Retries left: %d. " +
				"Timeout in: %d seconds", successfulPods, attemptsLeft, timeout)
			if successfulPods == 1 {
				logrus.Info("Provisioning job for " + name + " successfully completed")
				break
			}
			time.Sleep(5 * time.Second)
		}
	}
}