//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package util

import (
	"bytes"
	"fmt"
	"io"

	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type k8s struct {
	clientset kubernetes.Interface
}

func GetK8Client() *k8s {
	tests := IsTestMode()
	if !tests {
		cfg, err := config.GetConfig()
		if err != nil {
			logrus.Errorf(err.Error())
		}
		client := k8s{}
		client.clientset, err = kubernetes.NewForConfig(cfg)

		if err != nil {
			logrus.Errorf(err.Error())
			return nil
		}
		return &client
	}
	return nil
}

// GetPVCStatus waits for pvc.status.phase to be Bound
func (cl *k8s) GetPVCStatus(pvc *corev1.PersistentVolumeClaim, ns string) {
	// short timeout if a PVC is waiting for a first consumer to be bound
	var timeout int64 = 10
	listOptions := metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", pvc.Name).String(),
		TimeoutSeconds: &timeout,
	}
	watcher, err := cl.clientset.CoreV1().PersistentVolumeClaims(ns).Watch(listOptions)
	if err != nil {
		log.Error(err, "An error occurred")
	}
	ch := watcher.ResultChan()
	logrus.Infof("Waiting for PVC %s to be bound. Default timeout: %v seconds", pvc.Name, timeout)

	for event := range ch {
		pvc, ok := event.Object.(*corev1.PersistentVolumeClaim)
		if !ok {
			log.Error(err, "Unexpected type")
		}

		// check before watching in case pvc has been already bound
		chePvc, err := cl.clientset.CoreV1().PersistentVolumeClaims(ns).Get(pvc.Name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get %s pvc: %s", chePvc.Name, err)
			break
		}
		if chePvc.Status.Phase == "Bound" {
			volumeName := chePvc.Spec.VolumeName
			logrus.Infof("PVC %s successfully bound to volume %s", chePvc.Name, volumeName)
			break
		}

		switch event.Type {
		case watch.Error:
			watcher.Stop()
		case watch.Modified:
			if chePvc.Status.Phase == "Bound" {
				volumeName := chePvc.Spec.VolumeName
				logrus.Infof("PVC %s successfully bound to volume %s", chePvc.Name, volumeName)
				watcher.Stop()
			}

		}
	}
	chePvc, err := cl.clientset.CoreV1().PersistentVolumeClaims(ns).Get(pvc.Name, metav1.GetOptions{})
	if chePvc.Status.Phase != "Bound" {
		currentPvcPhase := chePvc.Status.Phase
		logrus.Warnf("Timeout waiting for a PVC %s to be bound. Current phase is %s", chePvc.Name, currentPvcPhase)
		logrus.Warn("Sometimes PVC can be bound only when the first consumer is created")
	}
}

func (cl *k8s) GetDeploymentRollingUpdateStatus(name string, ns string) {
	api := cl.clientset.AppsV1()
	var timeout int64 = 420
	listOptions := metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", name).String(),
		TimeoutSeconds: &timeout,
	}
	watcher, err := api.Deployments(ns).Watch(listOptions)
	if err != nil {
		log.Error(err, "An error occurred")
	}
	ch := watcher.ResultChan()
	logrus.Infof("Waiting for a successful rolling update of deployment %s. Default timeout: %v seconds", name, timeout)
	for event := range ch {
		dc, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			log.Error(err, "Unexpected type")
		}
		// check before watching in case the deployment is already scaled to 1
		deployment, err := cl.clientset.AppsV1().Deployments(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get %s deployment: %s", deployment.Name, err)
			break
		}
		if deployment.Status.Replicas == 1 {
			logrus.Infof("Rolling update of '%s' deployment finished", deployment.Name)
			break
		}
		switch event.Type {
		case watch.Error:
			watcher.Stop()
		case watch.Modified:
			if dc.Status.Replicas == 1 {
				logrus.Infof("Rolling update of '%s' deployment finished", deployment.Name)
				watcher.Stop()
			}

		}
	}
}

// GetDeploymentStatus listens to deployment events and checks replicas once MODIFIED event is received
func (cl *k8s) GetDeploymentStatus(name string, ns string) (scaled bool) {
	api := cl.clientset.AppsV1()
	var timeout int64 = 420
	listOptions := metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector("metadata.name", name).String(),
		TimeoutSeconds: &timeout,
	}
	watcher, err := api.Deployments(ns).Watch(listOptions)
	if err != nil {
		log.Error(err, "An error occurred")
	}
	ch := watcher.ResultChan()
	logrus.Infof("Waiting for deployment %s. Default timeout: %v seconds", name, timeout)
	for event := range ch {
		dc, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			log.Error(err, "Unexpected type")
		}
		// check before watching in case the deployment is already scaled to 1
		deployment, err := cl.clientset.AppsV1().Deployments(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get %s deployment: %s", deployment.Name, err)
			return false
		}
		if deployment.Status.AvailableReplicas == 1 {
			logrus.Infof("Deployment '%s' successfully scaled to %v", deployment.Name, deployment.Status.AvailableReplicas)
			return true
		}
		switch event.Type {
		case watch.Error:
			watcher.Stop()
		case watch.Modified:
			if dc.Status.AvailableReplicas == 1 {
				logrus.Infof("Deployment '%s' successfully scaled to %v", deployment.Name, dc.Status.AvailableReplicas)
				watcher.Stop()
				return true

			}
		}
	}
	dc, _ := cl.clientset.AppsV1().Deployments(ns).Get(name, metav1.GetOptions{})
	if dc.Status.AvailableReplicas != 1 {
		logrus.Errorf("Failed to verify a successful %s deployment", name)
		eventList := cl.GetEvents(name, ns).Items
		for i := range eventList {
			logrus.Errorf("Event message: %v", eventList[i].Message)
		}
		deploymentPod, err := cl.GetDeploymentPod(name, ns)
		if err != nil {
			return false
		}
		cl.GetPodLogs(deploymentPod, ns)
		logrus.Errorf("Command to get deployment logs: kubectl logs deployment/%s -n=%s", name, ns)
		logrus.Errorf("Get k8s events: kubectl get events "+
			"--field-selector "+
			"involvedObject.name=$(kubectl get pods -l=component=%s -n=%s"+
			" -o=jsonpath='{.items[0].metadata.name}') -n=%s", name, ns, ns)
		return false
	}
	return true
}

// GetEvents returns a list of events filtered by involvedObject
func (cl *k8s) GetEvents(deploymentName string, ns string) (list *corev1.EventList) {
	eventListOptions := metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("involvedObject.fieldPath", "spec.containers{"+deploymentName+"}").String()}
	deploymentEvents, _ := cl.clientset.CoreV1().Events(ns).List(eventListOptions)
	return deploymentEvents
}

func (cl *k8s) IsPVCExists(pvcName string, ns string) bool {
	getOptions := metav1.GetOptions{}
	_, err := cl.clientset.CoreV1().PersistentVolumeClaims(ns).Get(pvcName, getOptions)
	return err == nil
}

func (cl *k8s) DeletePVC(pvcName string, ns string) {
	logrus.Infof("Deleting PVC: %s", pvcName)
	deleteOptions := &metav1.DeleteOptions{}
	err := cl.clientset.CoreV1().PersistentVolumeClaims(ns).Delete(pvcName, deleteOptions)
	if err != nil {
		logrus.Errorf("PVC deletion error: %v", err)
	}
}

func (cl *k8s) IsDeploymentExists(deploymentName string, ns string) bool {
	getOptions := metav1.GetOptions{}
	_, err := cl.clientset.AppsV1().Deployments(ns).Get(deploymentName, getOptions)
	return err == nil
}

func (cl *k8s) DeleteDeployment(deploymentName string, ns string) {
	logrus.Infof("Deleting deployment: %s", deploymentName)
	deleteOptions := &metav1.DeleteOptions{}
	err := cl.clientset.AppsV1().Deployments(ns).Delete(deploymentName, deleteOptions)
	if err != nil {
		logrus.Errorf("Deployment deletion error: %v", err)
	}
}

// GetLogs prints stderr or stdout from a selected pod. Log size is capped at 60000 bytes
func (cl *k8s) GetPodLogs(podName string, ns string) {
	var limitBytes int64 = 60000
	req := cl.clientset.CoreV1().Pods(ns).GetLogs(podName, &corev1.PodLogOptions{LimitBytes: &limitBytes})
	readCloser, err := req.Stream()
	if err != nil {
		logrus.Errorf("Pod error log: %v", err)
	} else {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, readCloser)
		logrus.Infof("Pod log: %v", buf.String())
	}
}

//GetDeploymentPod queries all pods is a selected namespace by LabelSelector
func (cl *k8s) GetDeploymentPod(name string, ns string) (podName string, err error) {
	api := cl.clientset.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "component=" + name,
	}
	podList, _ := api.Pods(ns).List(listOptions)
	podListItems := podList.Items
	if len(podListItems) == 0 {
		logrus.Errorf("Failed to find pod to exec into. List of pods: %v", podListItems)
		return "", err
	}
	// expecting only one pod to be there so, taking the first one
	// todo maybe add a unique label to deployments?
	podName = podListItems[0].Name
	return podName, nil
}

// Reads 'user' and 'password' from the given secret
func (cl *k8s) ReadSecret(name string, ns string) (user string, password string, err error) {
	secret, err := cl.clientset.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	return string(secret.Data["user"]), string(secret.Data["password"]), nil
}

func (cl *k8s) RunExec(command []string, podName, namespace string) (string, string, error) {

	req := cl.clientset.CoreV1().RESTClient().Post().
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

	cfg, _ := config.GetConfig()
	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
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
