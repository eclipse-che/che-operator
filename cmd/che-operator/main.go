package main

import (
	"github.com/eclipse/che-operator/pkg/operator"
	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	route "github.com/openshift/api/route/v1"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	rbac "k8s.io/api/rbac/v1"
	"runtime"
	"strings"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func init() {
	logrus.Info("Operator is running on: ", strings.ToUpper(util.GetInfra()))
	k8sutil.AddToSDKScheme(appsv1.AddToScheme)
	k8sutil.AddToSDKScheme(rbac.AddToScheme)
	k8sutil.AddToSDKScheme(route.AddToScheme)
	k8sutil.AddToSDKScheme(oauth.AddToScheme)
	k8sutil.AddToSDKScheme(batchv1.AddToScheme)

}

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func main() {
	printVersion()
	operator.ReconsileChe()
}
