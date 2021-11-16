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
package util

import (
	"bytes"
	"context"
	"fmt"
	"io"

	v1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/sirupsen/logrus"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

func (cl *k8s) ExecIntoPod(
	cr *v1.CheCluster,
	deploymentName string,
	getCommand func(*v1.CheCluster) (string, error),
	reason string) (string, error) {

	command, err := getCommand(cr)
	if err != nil {
		return "", err
	}

	pod, err := cl.GetDeploymentPod(deploymentName, cr.Namespace)
	if err != nil {
		return "", err
	}

	return cl.DoExecIntoPod(cr.Namespace, pod, command, reason)
}

func (cl *k8s) DoExecIntoPod(namespace string, podName string, command string, reason string) (string, error) {
	var stdin io.Reader
	return cl.DoExecIntoPodWithStdin(namespace, podName, command, stdin, reason)
}

func (cl *k8s) DoExecIntoPodWithStdin(namespace string, podName string, command string, stdin io.Reader, reason string) (string, error) {
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
func (cl *k8s) GetDeploymentPod(name string, ns string) (podName string, err error) {
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

func (cl *k8s) GetPodsByComponent(name string, ns string) []string {
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

// Reads 'user' and 'password' from the given secret
func (cl *k8s) ReadSecret(name string, ns string) (user string, password string, err error) {
	secret, err := cl.clientset.CoreV1().Secrets(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	return string(secret.Data["user"]), string(secret.Data["password"]), nil
}

func (cl *k8s) RunExec(command []string, podName, namespace string, stdin io.Reader) (string, string, error) {

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

func (cl *k8s) IsResourceOperationPermitted(resourceAttr *authorizationv1.ResourceAttributes) (ok bool, err error) {
	lsar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: resourceAttr,
		},
	}

	ssar, err := cl.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(context.TODO(), lsar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	return ssar.Status.Allowed, nil
}

func InNamespaceEventFilter(namespace string) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			return namespace == ce.Object.GetNamespace()
		},
		DeleteFunc: func(de event.DeleteEvent) bool {
			return namespace == de.Object.GetNamespace()
		},
		UpdateFunc: func(ue event.UpdateEvent) bool {
			return namespace == ue.ObjectOld.GetNamespace()
		},
		GenericFunc: func(ge event.GenericEvent) bool {
			return namespace == ge.Object.GetNamespace()
		},
	}
}
