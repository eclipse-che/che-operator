//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package proxyconfig

import (
	"fmt"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/cheproxy"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ProxyConfigReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *ProxyConfigReconciler {
	return &ProxyConfigReconciler{}
}

func (r *ProxyConfigReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := r.discover(cheCtx); err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to discover proxy config: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (r *ProxyConfigReconciler) discover(cheCtx *chetypes.CheContext) error {
	cheCtx.Proxy = &cheproxy.Proxy{}

	if infrastructure.IsOpenShift() {
		return r.discoverOnOpenShift(cheCtx)
	}
	return r.discoverOnKubernetes(cheCtx)
}

func (r *ProxyConfigReconciler) Finalize(_ *chetypes.CheContext) error {
	return nil
}

func (r *ProxyConfigReconciler) discoverOnOpenShift(cheCtx *chetypes.CheContext) error {
	cheConfig := cheCtx.CheCluster.Spec.Components.CheServer.Proxy
	cheProxy, err := buildFromCheConfig(cheCtx, cheConfig)
	if err != nil {
		return err
	}

	clusterConfig := &configv1.Proxy{}
	_, err = resources.Proxy.Get(cheCtx, clusterConfig)
	if err != nil {
		return err
	}
	clusterProxy, err := buildFromClusterConfig(clusterConfig)
	if err != nil {
		return err
	}

	// If proxy configuration exists in CR then cluster wide proxy configuration is ignored.
	if cheProxy.IsPresent() {
		cheCtx.Proxy = cheProxy
	} else if clusterProxy.IsPresent() {
		cheCtx.Proxy = clusterProxy
	}

	// Always store certificate ConfigMap, see `certificates.go`
	cheCtx.Proxy.TrustedCAMapName = clusterConfig.Spec.TrustedCA.Name

	if cheCtx.Proxy.IsPresent() {
		// Merge all non-proxy hosts lists
		cheCtx.Proxy.NoProxy = mergeNoProxyHosts(os.Getenv("KUBERNETES_SERVICE_HOST"), ".svc", cheProxy.NoProxy, clusterConfig.Status.NoProxy)
	}

	return nil
}

func (r *ProxyConfigReconciler) discoverOnKubernetes(cheCtx *chetypes.CheContext) error {
	cheConfig := cheCtx.CheCluster.Spec.Components.CheServer.Proxy

	cheProxy, err := buildFromCheConfig(cheCtx, cheConfig)
	if err != nil {
		return err
	}
	if cheProxy.IsPresent() {
		cheProxy.NoProxy = mergeNoProxyHosts(os.Getenv("KUBERNETES_SERVICE_HOST"), ".svc", cheProxy.NoProxy)

		cheCtx.Proxy = cheProxy
		return nil
	}

	return nil
}

func buildFromCheConfig(cheCtx *chetypes.CheContext, cheConfig *chev2.Proxy) (*cheproxy.Proxy, error) {
	if cheConfig == nil {
		return &cheproxy.Proxy{}, nil
	}

	var host string
	var protocol string
	var username string
	var password string

	proxyParts := strings.SplitN(cheConfig.Url, "://", 2)

	if len(proxyParts) == 1 {
		host = proxyParts[0]
	} else {
		protocol = proxyParts[0]
		host = proxyParts[1]
	}

	credentialsSecret := &corev1.Secret{}
	secretName := utils.ValueOrDef(cheConfig.CredentialsSecretName, constants.DefaultProxyCredentialsSecret)

	exists, err := resources.Secret.Get(cheCtx, secretName, cheCtx.CheCluster.Namespace, credentialsSecret)
	if err != nil {
		return nil, err
	}
	if exists {
		username = string(credentialsSecret.Data["user"])
		password = string(credentialsSecret.Data["password"])
	}

	proxyURL := host
	if cheConfig.Port != "" {
		proxyURL = fmt.Sprintf("%s:%s", proxyURL, cheConfig.Port)
	}
	if username != "" && password != "" {
		proxyURL = fmt.Sprintf("%s:%s@%s", username, password, proxyURL)
	}
	if protocol != "" {
		proxyURL = fmt.Sprintf("%s://%s", protocol, proxyURL)
	}

	return &cheproxy.Proxy{
		HttpProxy:    proxyURL,
		HttpHost:     host,
		HttpPort:     cheConfig.Port,
		HttpUser:     username,
		HttpPassword: password,

		HttpsProxy:    proxyURL,
		HttpsHost:     host,
		HttpsPort:     cheConfig.Port,
		HttpsUser:     username,
		HttpsPassword: password,

		NoProxy: strings.Join(cheConfig.NonProxyHosts, ","),
	}, nil
}

func buildFromClusterConfig(clusterConfig *configv1.Proxy) (*cheproxy.Proxy, error) {
	if clusterConfig.Status.HTTPProxy == "" && clusterConfig.Status.HTTPSProxy == "" {
		return &cheproxy.Proxy{}, nil
	}

	httpProxy := clusterConfig.Status.HTTPProxy
	httpsProxy := utils.ValueOrDef(clusterConfig.Status.HTTPSProxy, clusterConfig.Status.HTTPProxy)

	httpProxyURL, err := url.Parse(httpProxy)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy config: %w", err)
	}
	httpsProxyURL, err := url.Parse(httpsProxy)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy config: %w", err)
	}

	getPwd := func(ui *url.Userinfo) string {
		pwd, _ := ui.Password()
		return pwd
	}

	return &cheproxy.Proxy{
		HttpProxy:    httpProxy,
		HttpHost:     httpProxyURL.Hostname(),
		HttpPort:     httpProxyURL.Port(),
		HttpUser:     httpProxyURL.User.Username(),
		HttpPassword: getPwd(httpProxyURL.User),

		HttpsProxy:    httpsProxy,
		HttpsHost:     httpsProxyURL.Hostname(),
		HttpsPort:     httpsProxyURL.Port(),
		HttpsUser:     httpsProxyURL.User.Username(),
		HttpsPassword: getPwd(httpsProxyURL.User),

		NoProxy:          clusterConfig.Status.NoProxy,
		TrustedCAMapName: clusterConfig.Spec.TrustedCA.Name,
	}, nil
}

func mergeNoProxyHosts(lists ...string) string {
	unique := make(map[string]struct{})
	for _, list := range lists {
		for host := range strings.SplitSeq(list, ",") {
			if host = strings.TrimSpace(host); host != "" {
				unique[host] = struct{}{}
			}
		}
	}
	return strings.Join(slices.Sorted(maps.Keys(unique)), ",")
}
