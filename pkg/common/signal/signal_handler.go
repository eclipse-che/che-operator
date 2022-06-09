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

package signal

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGINT}
var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandler set up custom signal handler for main process.
func SetupSignalHandler(terminationPeriod int64) context.Context {
	logrus.Info("Set up process signal handler")
	close(onlyOneSignalHandler) // panics when called twice

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		sig := <-c
		printSignal(sig)

		// We need provide more time for operator controllers go routing to complete finalizers logic.
		// Otherwise resource won't be clean up gracefully
		// and Che custom resource will stay with non empty "finalizers" field.
		time.Sleep(time.Duration(terminationPeriod) * time.Second)
		logrus.Info("Stop and exit operator.")
		// Stop Che controller
		cancel()
		<-c
		// Second signal. Exit from main process directly.
		os.Exit(1)
	}()

	return ctx
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
	cheFlavor := defaults.GetCheFlavor()
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
