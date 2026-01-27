//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"time"

	dwInfra "github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1 "github.com/openshift/api/user/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	workspaceconfig "github.com/eclipse-che/che-operator/controllers/workspaceconfig"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	dwr "github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting"

	"github.com/eclipse-che/che-operator/controllers/devworkspace/solver"
	"github.com/eclipse-che/che-operator/controllers/usernamespace"

	securityv1 "github.com/openshift/api/security/v1"

	dwoApi "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"go.uber.org/zap/zapcore"

	"github.com/eclipse-che/che-operator/controllers/devworkspace"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/signal"
	"github.com/eclipse-che/che-operator/pkg/common/test"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/client-go/discovery"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	osruntime "runtime"

	"fmt"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	templatev1 "github.com/openshift/api/template/v1"

	checontroller "github.com/eclipse-che/che-operator/controllers/che"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"

	imagepullerapi "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	projectv1 "github.com/openshift/api/project/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
)

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string

	leaseDuration = 40 * time.Second
	renewDeadline = 30 * time.Second
)

func init() {
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

	defaults.Initialize()

	printVersion(logger)

	utilruntime.Must(chev1.AddToScheme(scheme))
	utilruntime.Must(chev2.AddToScheme(scheme))

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(admissionregistrationv1.AddToScheme(scheme))
	utilruntime.Must(rbacv1.AddToScheme(scheme))

	utilruntime.Must(imagepullerapi.AddToScheme(scheme))
	utilruntime.Must(packagesv1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))

	if infrastructure.IsOpenShift() {
		utilruntime.Must(routev1.Install(scheme))
		utilruntime.Must(configv1.Install(scheme))
		utilruntime.Must(consolev1.Install(scheme))
		utilruntime.Must(projectv1.Install(scheme))
		utilruntime.Must(securityv1.Install(scheme))
		utilruntime.Must(templatev1.Install(scheme))
	}

	// User and OAuthClient API are disabled in case of external IDP
	// Check API before adding to the scheme
	if infrastructure.IsOpenShiftOAuthEnabled() {
		utilruntime.Must(userv1.Install(scheme))
		utilruntime.Must(oauthv1.Install(scheme))
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
	if infrastructure.IsOpenShift() {
		infra = "OpenShift"
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
	if err := dwInfra.Initialize(); err != nil {
		setupLog.Error(err, "Failed to initialize infrastructure")
		os.Exit(1)
	}

	watchNamespace, err := getWatchNamespace()
	if err != nil {
		setupLog.Error(err, "unable to get WatchNamespace, "+
			"the manager will watch and manage resources in all namespaces")
	}

	config := ctrl.GetConfigOrDie()

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		setupLog.Error(err, "failed to create discovery client")
		os.Exit(1)
	}

	if !infrastructure.IsLeaderElectionEnabled() {
		setupLog.Info("Leader election disabled")
		enableLeaderElection = false
	}

	// Add the Dev Workspace API to the scheme
	if err := dwoApi.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Dev Workspace Operator is not installed")
		os.Exit(1)
	}

	cacheFunction, err := getCacheFunc()
	if err != nil {
		setupLog.Error(err, "failed to create cache function")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       server.Options{BindAddress: metricsAddr},
		WebhookServer:                 webhook.NewServer(webhook.Options{Port: 9443}),
		HealthProbeBindAddress:        probeAddr,
		LeaderElection:                enableLeaderElection,
		LeaderElectionID:              "e79b08a4.org.eclipse.che",
		LeaderElectionReleaseOnCancel: true,
		LeaseDuration:                 &leaseDuration,
		RenewDeadline:                 &renewDeadline,
		NewCache:                      cacheFunction,

		// NOTE: We CANNOT limit the manager to a single namespace, because that would limit the
		// devworkspace routing reconciler to a single namespace, which would make it totally unusable.
		// Instead, if some controller wants to limit itself to single namespace, it can do it
		// for example using an event filter, as checontroller does.
		// Namespace:              watchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	nonCachingClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "unable to initialize non cached client")
		os.Exit(1)
	}

	// Setup all Controllers
	cheReconciler := checontroller.NewReconciler(mgr.GetClient(), nonCachingClient, discoveryClient, mgr.GetScheme(), watchNamespace)
	if err = cheReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "CheCluster")
		os.Exit(1)
	}

	routing := dwr.DevWorkspaceRoutingReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("DevWorkspaceRouting"),
		Scheme:       mgr.GetScheme(),
		SolverGetter: solver.Getter(mgr.GetScheme()),
	}
	if err := routing.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "DevWorkspaceRouting")
		os.Exit(1)
	}

	namespacecache := namespacecache.NewNamespaceCache(nonCachingClient)

	userNamespaceReconciler := usernamespace.NewCheUserNamespaceReconciler(mgr.GetClient(), nonCachingClient, mgr.GetScheme(), namespacecache)
	if err = userNamespaceReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "CheUserReconciler")
		os.Exit(1)
	}

	workspacesConfigReconciler := workspaceconfig.NewWorkspacesConfigReconciler(mgr.GetClient(), nonCachingClient, mgr.GetScheme(), namespacecache)
	if err = workspacesConfigReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up controller", "controller", "WorkspacesConfigReconciler")
		os.Exit(1)
	}

	terminationPeriod := int64(20)
	if !test.IsTestMode() {
		namespace, err := infrastructure.GetOperatorNamespace()
		if err == nil {
			terminationPeriod = signal.GetTerminationGracePeriodSeconds(mgr.GetAPIReader(), namespace)
		}
	}
	sigHandler := signal.SetupSignalHandler(terminationPeriod)

	// we install the devworkspace CheCluster reconciler even if dw is not supported so that it
	// can write meaningful status messages into the CheCluster CRs.
	dwChe := devworkspace.CheClusterReconciler{}
	if err := dwChe.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to set up devWorkspace controller", "controller", "DevWorkspaceReconciler")
		os.Exit(1)
	}

	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		if err = chev2.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "CheCluster")
			os.Exit(1)
		}
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
	setupLog.Info("starting manager")
	if err := mgr.Start(sigHandler); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// watch k8s objects with labels `app.kubernetes.io/part-of=che.eclipse.org`
func getCacheFunc() (cache.NewCacheFunc, error) {
	partOfCheObjectSelector, err := labels.Parse(fmt.Sprintf("%s=%s", constants.KubernetesPartOfLabelKey, constants.CheEclipseOrg))
	if err != nil {
		return nil, err
	}

	selectors := map[client.Object]cache.ByObject{
		&appsv1.Deployment{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.Pod{}: {
			Label: partOfCheObjectSelector,
		},
		&batchv1.Job{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.Service{}: {
			Label: partOfCheObjectSelector,
		},
		&networkingv1.Ingress{}: {
			Label: partOfCheObjectSelector,
		},
		&networkingv1.NetworkPolicy{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.ConfigMap{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.Secret{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.ServiceAccount{}: {
			Label: partOfCheObjectSelector,
		},
		&rbacv1.Role{}: {
			Label: partOfCheObjectSelector,
		},
		&rbacv1.RoleBinding{}: {
			Label: partOfCheObjectSelector,
		},
		&rbacv1.ClusterRole{}: {
			Label: partOfCheObjectSelector,
		},
		&rbacv1.ClusterRoleBinding{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.PersistentVolumeClaim{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.LimitRange{}: {
			Label: partOfCheObjectSelector,
		},
		&corev1.ResourceQuota{}: {
			Label: partOfCheObjectSelector,
		},
	}

	if infrastructure.IsOpenShift() {
		selectors[&routev1.Route{}] = cache.ByObject{Label: partOfCheObjectSelector}
		selectors[&templatev1.Template{}] = cache.ByObject{Label: partOfCheObjectSelector}
	}

	if infrastructure.IsOpenShiftOAuthEnabled() {
		selectors[&oauthv1.OAuthClient{}] = cache.ByObject{Label: partOfCheObjectSelector}
	}

	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		opts.ByObject = selectors
		return cache.New(config, opts)
	}, nil
}
