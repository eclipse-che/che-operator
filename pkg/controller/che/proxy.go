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
	proxy, err := deploy.ReadCheClusterProxyConfiguration(checluster)
	if err != nil {
		return nil, err
	}

	if util.IsOpenshift4() {
		clusterProxy := &configv1.Proxy{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, clusterProxy); err != nil {
			return nil, err
		}

		// If proxy configuration exists in CR then cluster wide proxy configuration is ignored
		// otherwise cluster wide proxy configuration is used and non proxy hosts
		// are merged with defined ones in CR
		if proxy.HttpProxy == "" && clusterProxy.Status.HTTPProxy != "" {
			proxy, err = deploy.ReadClusterWideProxyConfiguration(clusterProxy, proxy.NoProxy)
			if err != nil {
				return nil, err
			}
		}
	}

	return proxy, nil
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
