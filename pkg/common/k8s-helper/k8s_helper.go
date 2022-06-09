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
package k8shelper

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type K8sHelper struct {
	clientset kubernetes.Interface
}

var (
	k8sHelper *K8sHelper
)

func New() *K8sHelper {
	if k8sHelper != nil {
		return k8sHelper
	}

	if test.IsTestMode() {
		return initializeForTesting()
	}

	return initialize()
}

func (cl *K8sHelper) GetClientset() kubernetes.Interface {
	return cl.clientset
}

func (cl *K8sHelper) ExecIntoPod(
	deploymentName string,
	command string,
	reason string,
	namespace string) (string, error) {
	pod, err := cl.GetDeploymentPod(deploymentName, namespace)
	if err != nil {
		return "", err
	}

	return cl.DoExecIntoPod(namespace, pod, command, reason)
}

func (cl *K8sHelper) DoExecIntoPod(namespace string, podName string, command string, reason string) (string, error) {
	var stdin io.Reader
	return cl.DoExecIntoPodWithStdin(namespace, podName, command, stdin, reason)
}

func (cl *K8sHelper) DoExecIntoPodWithStdin(namespace string, podName string, command string, stdin io.Reader, reason string) (string, error) {
	if reason != "" {
		logrus.Infof("Running exec for '%s' in the pod '%s'", reason, podName)
	}

	args := []string{"/bin/bash", "-c", command}
	stdout, stderr, err := cl.RunExec(args, podName, namespace, stdin)
	if err != nil {
		logrus.Errorf("Error running exec: %v, command: %s", err, args)
		if stderr != "" {
			logrus.Errorf("Stderr: %s", stderr)
		}
		return stdout, err
	}

	if reason != "" {
		logrus.Info("Exec successfully completed.")
	}
	return stdout, nil
}

//GetDeploymentPod queries all pods is a selected namespace by LabelSelector
func (cl *K8sHelper) GetDeploymentPod(name string, ns string) (podName string, err error) {
	api := cl.clientset.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "component=" + name,
	}
	podList, _ := api.Pods(ns).List(context.TODO(), listOptions)
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

func (cl *K8sHelper) GetPodsByComponent(name string, ns string) []string {
	names := []string{}
	api := cl.clientset.CoreV1()
	listOptions := metav1.ListOptions{
		LabelSelector: "component=" + name,
	}
	podList, _ := api.Pods(ns).List(context.TODO(), listOptions)
	for _, pod := range podList.Items {
		names = append(names, pod.Name)
	}

	return names
}

func (cl *K8sHelper) RunExec(command []string, podName, namespace string, stdin io.Reader) (string, string, error) {
	req := cl.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: command,
		Stdin:   stdin != nil,
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
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	return stdout.String(), stderr.String(), err
}

func initializeForTesting() *K8sHelper {
	k8sHelper = &K8sHelper{
		clientset: fake.NewSimpleClientset(),
	}

	return k8sHelper
}

func initialize() *K8sHelper {
	cfg, err := config.GetConfig()
	if err != nil {
		logrus.Fatalf("Failed to initialized Kubernetes client: %v", err)
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Failed to initialized Kubernetes client: %v", err)
	}

	k8sHelper = &K8sHelper{
		clientset: clientSet,
	}

	return k8sHelper
}
