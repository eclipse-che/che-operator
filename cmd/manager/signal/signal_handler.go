//
// Copyright (c) 2012-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package signal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupSignalHandler set up custom signal handler for main process.
func SetupSignalHandler(terminationPeriod int64) (stopCh <-chan struct{}) {
	logrus.Info("Set up process signal handler")
	var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGINT}

	stop := make(chan struct{})
	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func() {
		sig := <-c
		printSignal(sig)

		// We need provide more time for Che controller go routing to complete finalizers logic.
		// Otherwise resource won't be clean up gracefully
		// and Che custom resource will stay with non empty "finalizers" field.
		time.Sleep(time.Duration(terminationPeriod) * time.Second)
		logrus.Info("Stop and exit operator.")
		// Stop Che controller
		close(stop)
		// Exit from main process directly.
		os.Exit(1)
	}()

	return stop
}

func printSignal(signal os.Signal) {
	switch signal {
	case syscall.SIGHUP:
		logrus.Info("Signal SIGHUP")

	case syscall.SIGINT:
		logrus.Println("Signal SIGINT (ctrl+c)")

	case syscall.SIGTERM:
		logrus.Println("Signal SIGTERM stop")

	case syscall.SIGQUIT:
		logrus.Println("Signal SIGQUIT (top and core dump)")

	default:
		logrus.Println("Unknown signal")
	}
}

func GetTerminationGracePeriodSeconds(k8sClient client.Reader, namespace string) int64 {
	cheFlavor := os.Getenv("CHE_FLAVOR")
	if cheFlavor == "" {
		cheFlavor = "che"
	}
	defaultTerminationGracePeriodSeconds := int64(20)

	deployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: cheFlavor + "-operator"}
	if err := k8sClient.Get(context.TODO(), namespacedName, deployment); err != nil {
		logrus.Warnf("Unable to find '%s' deployment in namespace '%s', err: %s", cheFlavor+"-operator", namespace, err.Error())
	} else {
		terminationPeriod := deployment.Spec.Template.Spec.TerminationGracePeriodSeconds
		if terminationPeriod != nil {
			logrus.Infof("Use 'terminationGracePeriodSeconds' %d sec. from operator deployment.", *terminationPeriod)
			return *terminationPeriod
		}
	}

	logrus.Infof("Use default 'terminationGracePeriodSeconds' %d sec.", defaultTerminationGracePeriodSeconds)
	return defaultTerminationGracePeriodSeconds
}
