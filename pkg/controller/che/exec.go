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
package che

import (
	"bytes"
	"fmt"
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/sirupsen/logrus"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func ExecIntoPod(podName string, provisionCommand string, reason string, ns string) (provisioned bool) {

	command := []string{"/bin/bash", "-c", provisionCommand}
	logrus.Infof("Running exec to %s in pod %s", reason, podName)
	// print std if operator is run in debug mode (TODO)
	_, stderr, err := k8sclient.RunExec(command, podName, ns)
	if err != nil {
		logrus.Errorf("Error exec'ing into pod: %v: , command: %s", err, command)
		logrus.Errorf(stderr)
		return false
	}
	logrus.Info("Exec successfully completed")
	return true
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

func (r *ReconcileChe) CreateKeycloakResources(instance *orgv1.CheCluster, request reconcile.Request, deploymentName string) (err error) {
	cheHost := instance.Spec.Server.CheHost
	keycloakProvisionCommand := deploy.GetKeycloakProvisionCommand(instance, cheHost)
	podToExec, err := k8sclient.GetDeploymentPod(deploymentName, instance.Namespace)
	if err != nil {
		logrus.Errorf("Failed to retrieve pod name. Further exec will fail")
	}
	provisioned := ExecIntoPod(podToExec, keycloakProvisionCommand, "create realm, client and user", instance.Namespace)
	if provisioned {
		instance, err := r.GetCR(request)
		if err != nil {
			if errors.IsNotFound(err) {
				logrus.Errorf("CR %s not found: %s", instance.Name, err)
				return err
			}
			logrus.Errorf("Error when getting %s CR: %s", instance.Name, err)
			return err
		}
		for {
			instance.Status.KeycloakProvisoned = true
			if err := r.UpdateCheCRStatus(instance, "status: provisioned with Keycloak", "true"); err != nil &&
			errors.IsConflict(err) {
				instance, _ = r.GetCR(request)
				continue
			}
			break
		}

		return nil
	}
	return err
}
