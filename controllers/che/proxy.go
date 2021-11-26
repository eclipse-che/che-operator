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
package che

import (
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
)

func GetProxyConfiguration(deployContext *deploy.DeployContext) (*deploy.Proxy, error) {
	// OpenShift 4.x
	if util.IsOpenShift4 {
		clusterProxy := &configv1.Proxy{}
		exists, err := deploy.GetClusterObject(deployContext, "cluster", clusterProxy)
		if err != nil {
			return nil, err
		}

		clusterWideProxyConf := &deploy.Proxy{}
		if exists {
			clusterWideProxyConf, err = deploy.ReadClusterWideProxyConfiguration(clusterProxy)
			if err != nil {
				return nil, err
			}
		}

		cheClusterProxyConf, err := deploy.ReadCheClusterProxyConfiguration(deployContext.CheCluster)
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
			return cheClusterProxyConf, nil
		} else {
			clusterWideProxyConf.NoProxy = deploy.MergeNonProxy(clusterWideProxyConf.NoProxy, cheClusterProxyConf.NoProxy)
			return clusterWideProxyConf, nil
		}
	}

	// OpenShift 3.x and k8s
	cheClusterProxyConf, err := deploy.ReadCheClusterProxyConfiguration(deployContext.CheCluster)
	if err != nil {
		return nil, err
	}
	if deployContext.CheCluster.IsInternalClusterSVCNamesEnabled() {
		cheClusterProxyConf.NoProxy = deploy.MergeNonProxy(cheClusterProxyConf.NoProxy, ".svc")
	}
	return cheClusterProxyConf, nil
}
