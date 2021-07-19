//
// Copyright (c) 2012-2021 Red Hat, Inc.
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
	"flag"
	"os"

	"go.uber.org/zap/zapcore"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	osruntime "runtime"

	"fmt"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	oauth "github.com/openshift/api/oauth/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	checontroller "github.com/eclipse-che/che-operator/controllers/che"
	backupcontroller "github.com/eclipse-che/che-operator/controllers/checlusterbackup"
	restorecontroller "github.com/eclipse-che/che-operator/controllers/checlusterrestore"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/signal"
	"github.com/eclipse-che/che-operator/pkg/util"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	rbac "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	image_puller_api "github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	corev1 "k8s.io/api/core/v1"

	orgv2alpha1 "github.com/eclipse-che/che-operator/api/v2alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	defaultsPath         string
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
)

func init() {
	flag.StringVar(&defaultsPath, "defaults-path", "", "Path to file with operator deployment defaults. This option is useful for local development.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":60000", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":6789", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	opts := zap.Options{
		Development: true,
		Level:       getLogLevel(),
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	logger := zap.New(zap.UseFlagOptions(&opts))
	ctrl.SetLogger(logger)

	deploy.InitDefaults(defaultsPath)

	if _, _, err := util.DetectOpenShift(); err != nil {
		logger.Error(err, "Unable determine installation platform")
		os.Exit(1)
	}

	printVersion(logger)

	// Uncomment when orgv2alpha1 will be ready
	// utilruntime.Must(orgv2alpha1.AddToScheme(scheme))

	utilruntime.Must(orgv2alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))

	// Setup Scheme for all resources
	utilruntime.Must(orgv1.AddToScheme(scheme))
	utilruntime.Must(image_puller_api.AddToScheme(scheme))
	utilruntime.Must(packagesv1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1.AddToScheme(scheme))

	if util.IsOpenShift {
		utilruntime.Must(routev1.AddToScheme(scheme))
		utilruntime.Must(oauth.AddToScheme(scheme))
		utilruntime.Must(userv1.AddToScheme(scheme))
		utilruntime.Must(configv1.AddToScheme(scheme))
		utilruntime.Must(corev1.AddToScheme(scheme))
		utilruntime.Must(consolev1.AddToScheme(scheme))
	}
}

func getLogLevel() zapcore.Level {
	switch logLevel, _ := os.LookupEnv("LOG_LEVEL"); logLevel {
	case zapcore.DebugLevel.String():
		return zapcore.DebugLevel
	case zapcore.InfoLevel.String():
		return zapcore.InfoLevel
	case zapcore.WarnLevel.String():
		return zapcore.WarnLevel
	case zapcore.ErrorLevel.String():
		return zapcore.ErrorLevel
	case zapcore.PanicLevel.String():
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

func printVersion(logger logr.Logger) {
	logger.Info("Binary info ", "Go version", osruntime.Version())
	logger.Info("Binary info ", "OS", osruntime.GOOS, "Arch", osruntime.GOARCH)
	logger.Info("Address ", "Metrics", metricsAddr)
	logger.Info("Address ", "Probe", probeAddr)

	infra := "Kubernetes"
	if util.IsOpenShift {
		infra = "OpenShift"
		if util.IsOpenShift4 {
			infra += " v4.x"
		} else {
			infra += " v3.x"
		}
	}
	logger.Info("Operator is running on ", "Infrastructure", infra)
}

// getWatchNamespace returns the Namespace the operator should be watching for changes
func getWatchNamespace() (string, error) {
	// WatchNamespaceEnvVar is the constant for env variable WATCH_NAMESPACE
	// which specifies the Namespace to watch.
	// An empty value means the operator is running with cluster scope.
	var watchNamespaceEnvVar = "WATCH_NAMESPACE"

	ns, found := os.LookupEnv(watchNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", watchNamespaceEnvVar)
	}

	return ns, nil
}

func main() {
	watchNamespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, "unable to get WatchNamespace, "+
			"the manager will watch and manage resources in all namespaces")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "e79b08a4.org.eclipse.che",
		Namespace:              watchNamespace,
		// TODO try to use it instead of signal handler....
		// GracefulShutdownTimeout: ,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cheReconciler, err := checontroller.NewReconciler(mgr)
	if err != nil {
		setupLog.Error(err, "unable to create checluster reconciler")
		os.Exit(1)
	}
	backupReconciler := backupcontroller.NewReconciler(mgr)
	restoreReconciler := restorecontroller.NewReconciler(mgr)

	if err = cheReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "CheCluster")
		os.Exit(1)
	}
	if err = backupReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "CheClusterBackup")
		os.Exit(1)
	}
	if err = restoreReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "CheClusterRestore")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Start the Cmd
	period := signal.GetTerminationGracePeriodSeconds(mgr.GetAPIReader(), watchNamespace)
	setupLog.Info("starting manager")
	if err := mgr.Start(signal.SetupSignalHandler(period)); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
