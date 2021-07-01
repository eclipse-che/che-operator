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
package main

import (
	"context"
	"flag"
	"fmt"

	"os"
	"runtime"

	image_puller_api "github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis"
	"github.com/eclipse-che/che-operator/cmd/manager/signal"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"

	"github.com/eclipse-che/che-operator/pkg/apis"
	"github.com/eclipse-che/che-operator/pkg/controller"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/ready"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	defaultsPath string
)

func init() {
	flag.StringVar(&defaultsPath, "defaults-path", "", "Path to file with operator deployment defaults. This option is useful for local development.")
}

func setLogLevel() {
	logLevel, isFound := os.LookupEnv("LOG_LEVEL")
	if isFound && len(logLevel) > 0 {
		parsedLevel, err := logrus.ParseLevel(logLevel)
		if err == nil {
			logrus.SetLevel(parsedLevel)
			logrus.Infof("Configured '%s' log level is applied", logLevel)
		} else {
			logrus.Errorf("Failed to parse log level `%s`. Possible values: panic, fatal, error, warn, info, debug. Default 'info' is applied", logLevel)
			logrus.SetLevel(logrus.InfoLevel)
		}
	} else {
		logrus.Infof("Default 'info' log level is applied")
		logrus.SetLevel(logrus.InfoLevel)
	}
}

func printVersion() {
	setLogLevel()
	logrus.Infof(fmt.Sprintf("Go Version: %s", runtime.Version()))
	logrus.Infof(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	logrus.Infof(fmt.Sprintf("operator-sdk Version: %v", sdkVersion.Version))
	isOpenShift, isOpenShift4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Fatalf("Operator is exiting. An error occurred when detecting current infra: %s", err)

	}
	infra := "Kubernetes"
	if isOpenShift {
		infra = "OpenShift"
		if isOpenShift4 {
			infra += " v4.x"
		} else {
			infra += " v3.x"
		}
	}
	logrus.Infof(fmt.Sprintf("Operator is running on %v", infra))
}

func main() {
	flag.Parse()
	deploy.InitDefaults(defaultsPath)
	printVersion()
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Errorf("Failed to get watch namespace. Using default namespace eclipse-che: %s", err)
		namespace = "eclipse-che"
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	r := ready.NewFileReady()
	err = r.Set()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	defer r.Unset()

	// Become the leader before proceeding
	leader.Become(context.TODO(), "che-operator-lock")

	// Create a new Cmd to provide shared dependencies and start components
	options := manager.Options{
		Namespace:              namespace,
		MetricsBindAddress:     ":8081",
		HealthProbeBindAddress: ":6789",
	}

	mgr, err := manager.New(cfg, options)
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	logrus.Info("Registering Che Components Types")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.Error(err, "")
		os.Exit(1)
	}

	if err := image_puller_api.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.Error(err, "")
		os.Exit(1)
	}

	if err := packagesv1.AddToScheme(mgr.GetScheme()); err != nil {
		logrus.Error(err, "")
		os.Exit(1)
	}

	if err := operatorsv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err := operatorsv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	logrus.Info("Starting the Cmd")

	// Start the Cmd
	period := signal.GetTerminationGracePeriodSeconds(mgr.GetAPIReader(), namespace)
	logrus.Info("Create manager")
	if err := mgr.Start(signal.SetupSignalHandler(period)); err != nil {
		logrus.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}
