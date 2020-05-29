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

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type k8s struct {
	clientset kubernetes.Interface
}

var (
	K8sclient = GetK8Client()
)

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

func (cl *k8s) ExecIntoPod(podName string, command string, reason string, namespace string) (string, error) {
	if reason != "" {
		logrus.Infof("Running exec for '%s' in the pod '%s'", reason, podName)
	}

	args := []string{"/bin/bash", "-c", command}
	stdout, stderr, err := cl.RunExec(args, podName, namespace)
	if err != nil {
		logrus.Errorf("Error running exec: %v, command: %s", err, args)
		logrus.Errorf("Stderr: %s", stderr)
		return stdout, err
	}

	if reason != "" {
		logrus.Info("Exec successfully completed.")
	}
	return stdout, nil
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
		logrus.Errorf("Failed to find pod for component %s. List of pods: %v", name, podListItems)
		return "", err
	}
	// expecting only one pod to be there so, taking the first one
	// todo maybe add a unique label to deployments?
	podName = podListItems[0].Name
	return podName, nil
}

func (cl *k8s) GetPodsByComponent(name string, ns string) []string {
	names := []string{}
	api := cl.clientset.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "component=" + name,
	}
	podList, _ := api.Pods(ns).List(listOptions)
	for _, pod := range podList.Items {
		names = append(names, pod.Name)
	}

	return names
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
