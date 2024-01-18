//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"os"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	configv1 "github.com/openshift/api/config/v1"
)

func GetProxyConfiguration(deployContext *chetypes.DeployContext) (*chetypes.Proxy, error) {
	if infrastructure.IsOpenShift() {
		clusterProxy := &configv1.Proxy{}
		exists, err := deploy.GetClusterObject(deployContext, "cluster", clusterProxy)
		if err != nil {
			return nil, err
		}

		clusterWideProxyConf := &chetypes.Proxy{}
		if exists {
			clusterWideProxyConf, err = deploy.ReadClusterWideProxyConfiguration(clusterProxy)
			if err != nil {
				return nil, err
			}
		}

		cheClusterProxyConf, err := deploy.ReadCheClusterProxyConfiguration(deployContext)
		if err != nil {
			return nil, err
		}

		// If proxy configuration exists in CR then cluster wide proxy configuration is ignored
		// Non proxy hosts are merged
		if cheClusterProxyConf.HttpProxy != "" {
			if clusterWideProxyConf.HttpProxy != "" {
				cheClusterProxyConf.NoProxy = deploy.MergeNonProxy(cheClusterProxyConf.NoProxy, clusterWideProxyConf.NoProxy)
			} else {
				cheClusterProxyConf.NoProxy = deploy.MergeNonProxy(cheClusterProxyConf.NoProxy, ".svc")
			}
			// Add cluster-wide trusted CA certs, if any
			cheClusterProxyConf.TrustedCAMapName = clusterWideProxyConf.TrustedCAMapName
			// Add kubernetes host to the no proxy list.
			cheClusterProxyConf.NoProxy = deploy.MergeNonProxy(cheClusterProxyConf.NoProxy, os.Getenv("KUBERNETES_SERVICE_HOST"))
			return cheClusterProxyConf, nil
		} else {
			if clusterWideProxyConf.HttpProxy != "" {
				// Add kubernetes host to the no proxy list.
				clusterWideProxyConf.NoProxy = deploy.MergeNonProxy(clusterWideProxyConf.NoProxy, os.Getenv("KUBERNETES_SERVICE_HOST"))
				clusterWideProxyConf.NoProxy = deploy.MergeNonProxy(clusterWideProxyConf.NoProxy, cheClusterProxyConf.NoProxy)
			}
			return clusterWideProxyConf, nil
		}
	}

	// OpenShift 3.x and k8s
	cheClusterProxyConf, err := deploy.ReadCheClusterProxyConfiguration(deployContext)
	if err != nil {
		return nil, err
	}
	if cheClusterProxyConf.HttpProxy != "" {
		// Add kubernetes host to the no proxy list.
		cheClusterProxyConf.NoProxy = deploy.MergeNonProxy(cheClusterProxyConf.NoProxy, os.Getenv("KUBERNETES_SERVICE_HOST"))
		cheClusterProxyConf.NoProxy = deploy.MergeNonProxy(cheClusterProxyConf.NoProxy, ".svc")
	}
	return cheClusterProxyConf, nil
}
