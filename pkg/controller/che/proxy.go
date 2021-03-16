//
// Copyright (c) 2020 Red Hat, Inc.
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
	"context"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/deploy/server"
	"github.com/eclipse/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileChe) getProxyConfiguration(checluster *orgv1.CheCluster) (*deploy.Proxy, error) {
	// OpenShift 4.x
	if util.IsOpenShift4 {
		clusterProxy := &configv1.Proxy{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, clusterProxy); err != nil {
			return nil, err
		}

		clusterWideProxyConf, err := deploy.ReadClusterWideProxyConfiguration(clusterProxy)
		if err != nil {
			return nil, err
		}

		cheClusterProxyConf, err := deploy.ReadCheClusterProxyConfiguration(checluster)
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
	cheClusterProxyConf, err := deploy.ReadCheClusterProxyConfiguration(checluster)
	if err != nil {
		return nil, err
	}
	if checluster.Spec.Server.UseInternalClusterSVCNames {
		cheClusterProxyConf.NoProxy = deploy.MergeNonProxy(cheClusterProxyConf.NoProxy, ".svc")
	}
	return cheClusterProxyConf, nil
}

func (r *ReconcileChe) putOpenShiftCertsIntoConfigMap(deployContext *deploy.DeployContext) (bool, error) {
	if deployContext.CheCluster.Spec.Server.ServerTrustStoreConfigMapName == "" {
		deployContext.CheCluster.Spec.Server.ServerTrustStoreConfigMapName = deploy.DefaultServerTrustStoreConfigMapName()
		if err := r.UpdateCheCRSpec(deployContext.CheCluster, "truststore configmap", deploy.DefaultServerTrustStoreConfigMapName()); err != nil {
			return false, err
		}
	}

	certConfigMap, err := server.SyncTrustStoreConfigMapToCluster(deployContext)
	return certConfigMap != nil, err
}
