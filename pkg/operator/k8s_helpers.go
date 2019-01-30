package operator

import (
	"bytes"
	"fmt"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"strconv"
)

type k8s struct {
	clientset kubernetes.Interface
}

func GetK8SConfig() *k8s {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	client := k8s{}
	client.clientset, err = kubernetes.NewForConfig(config)

	if err != nil {
		panic(err.Error())
		return nil
	}
	return &client
}
// GetDeploymentStatus listens to deployment events and checks replicas once MODIFIED event is received
func (o *k8s) GetDeploymentStatus(deployment *appsv1.Deployment)  {
	api := o.clientset.AppsV1()
	timeoutStr := util.GetEnv("WAIT_DEPLOYMENT_TIMEOUT", "420")
	timeout, err := strconv.ParseInt(timeoutStr, 10, 64)
	listOptions := metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", deployment.Name).String(),
		TimeoutSeconds: &timeout,
	}
	_, err = api.Deployments(namespace).Get(deployment.Name, metav1.GetOptions{})
	if err != nil {
		logrus.Fatalf("Failed to get %s deployment", deployment.Name)
	}
	watcher, err := api.Deployments(util.GetNamespace()).Watch(listOptions)
	if err != nil {
		log.Fatal(err)
	}
	ch := watcher.ResultChan()
	logrus.Printf("Waiting for deployment %s. Default timeout: %v seconds", deployment.Name, timeout)
	for event := range ch {
		dc, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			log.Fatal("Unexpected type")
		}

		//check before watching in case the deployment is already scaled to 1
		err := sdk.Get(deployment)
		if err != nil {
			logrus.Fatalf("Failed to get %s deployment: %s", deployment.Name, err)
		}
		if deployment.Status.AvailableReplicas == 1 {
			logrus.Infof("%s successfully deployed", deployment.Name)
			break
		}

		switch event.Type {
		case watch.Error:
			watcher.Stop()
		case watch.Modified:
			if dc.Status.AvailableReplicas == 1 {
				logrus.Infof("%s successfully deployed", deployment.Name)
				watcher.Stop()
			}
		}
	}

	if deployment.Status.AvailableReplicas != 1 {
		logrus.Errorf("Failed to verify a successful %s deployment. Operator is exiting", deployment.Name)
		eventList := o.GetEvents(deployment.Name).Items
		for i := range eventList {
			logrus.Errorf("Event message: %v", eventList[i].Message)
		}
		deploymentPod := o.GetDeploymentPod(deployment.Name)
		o.GetPodLogs(deploymentPod)
		logrus.Errorf("Command to get deployment logs: kubectl logs deployment/%s -n=%s", deployment.Name, deployment.Namespace)
		logrus.Fatalf("Get k8s events: kubectl get events "+
			"--field-selector "+
			"involvedObject.name=$(kubectl get pods -l=app=%s -n=%s"+
			" -o=jsonpath='{.items[0].metadata.name}') -n=%s", deployment.Name, deployment.Namespace, deployment.Namespace)

	}
}
// GetEvents returns a list of events filtered by involvedObject
func (o *k8s) GetEvents(deploymentName string) (list *corev1.EventList) {
	eventListOptions := metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("involvedObject.fieldPath", "spec.containers{"+deploymentName+"}").String()}
	deploymentEvents, _ := o.clientset.CoreV1().Events(namespace).List(eventListOptions)
	return deploymentEvents
}
// GetLogs prints stderr or stdout from a selected pod. Log size is capped at 60000 bytes
func (o *k8s) GetPodLogs(podName string) () {
	var limitBytes int64 = 60000
	req := o.clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{LimitBytes: &limitBytes},
	)
	readCloser, err := req.Stream()
	if err != nil {
		logrus.Errorf("Pod error log: %v", err)
	} else {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, readCloser)
		logrus.Infof("Pod log: %v", buf.String())
	}
}
// ExecToPod executes a remote command in a selected pod
func (o *k8s) ExecToPod(command []string, podName, namespace string) (string, string, error) {

	req := o.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, scheme.ParameterCodec)

	config, _ := rest.InClusterConfig()
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	var stdin io.Reader
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}

// GetDeploymentPod queries all pod is a selected namespace by LabelSelector
func (o *k8s) GetDeploymentPod(name string) (podName string) {
	api := o.clientset.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "app=" + name,
	}
	podList, _ := api.Pods(namespace).List(listOptions)
	podListItems := podList.Items
	if len(podListItems) == 0 {
		logrus.Error("Failed to find pod to exec into")
		logrus.Errorf("Pod list: %v ", podList.Items)
	}
	// expecting only one pod to be there
	podName = podListItems[0].Name
	return podName
}
