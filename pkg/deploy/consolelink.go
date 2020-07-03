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
package deploy

import (
	"context"
	"fmt"
	"strings"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var consoleLinkDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(consolev1.ConsoleLink{}, "TypeMeta"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return x.Name == y.Name
	}),
}

func SyncConsoleLinkToCluster(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) error {
	if !util.IsOpenShift4 || !HasConsolelinkObject() {
		logrus.Debug("Console link won't be created. It's not supported by cluster")
		// console link is supported only on OpenShift >= 4.2
		return nil
	}

	if !checluster.Spec.Server.TlsSupport {
		logrus.Debug("Console link won't be created. It's not supported when http connection is used")
		// console link is supported only with https
		return nil
	}

	specConsoleLink := getSpecConsoleLink(checluster)
	clusterConsoleLinks, err := getClusterConsoleLinks(checluster, clusterAPI.Client)
	if err != nil {
		return err
	}

	if !checluster.ObjectMeta.DeletionTimestamp.IsZero() {
		for _, clusterConsoleLink := range clusterConsoleLinks {
			logrus.Infof("Deleting existed object: %s, name %s", clusterConsoleLink.Kind, clusterConsoleLink.Name)
			return clusterAPI.Client.Delete(context.TODO(), clusterConsoleLink)
		}
		return nil
	}

	if len(clusterConsoleLinks) != 1 {
		for _, clusterConsoleLink := range clusterConsoleLinks {
			logrus.Infof("Deleting existed object: %s, name %s", clusterConsoleLink.Kind, clusterConsoleLink.Name)
			return clusterAPI.Client.Delete(context.TODO(), clusterConsoleLink)
		}

		logrus.Infof("Creating a new object: %s, name %s", specConsoleLink.Kind, specConsoleLink.Name)
		return clusterAPI.Client.Create(context.TODO(), specConsoleLink)
	}

	diff := cmp.Diff(clusterConsoleLinks[0], specConsoleLink, consoleLinkDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterConsoleLinks[0].Kind, clusterConsoleLinks[0].Name)
		fmt.Printf("Difference:\n%s", diff)

		err := clusterAPI.Client.Delete(context.TODO(), clusterConsoleLinks[0])
		if err != nil {
			return err
		}

		return clusterAPI.Client.Create(context.TODO(), specConsoleLink)
	}

	return nil
}

/**
 * Returns all console links for the same host name.
 */
func getClusterConsoleLinks(checluster *orgv1.CheCluster, client runtimeClient.Client) ([]*consolev1.ConsoleLink, error) {
	var clusterConsoleLinks []*consolev1.ConsoleLink

	consoleLinks := &consolev1.ConsoleLinkList{}
	listOptions := &runtimeClient.ListOptions{}
	if err := client.List(context.TODO(), listOptions, consoleLinks); err != nil {
		return nil, err
	}

	for _, consoleLink := range consoleLinks.Items {
		if strings.Contains(consoleLink.Spec.Link.Href, checluster.Spec.Server.CheHost) {
			clusterConsoleLinks = append(clusterConsoleLinks, &consoleLink)
		}
	}

	return clusterConsoleLinks, nil
}

func getSpecConsoleLink(checluster *orgv1.CheCluster) *consolev1.ConsoleLink {
	cheHost := checluster.Spec.Server.CheHost
	return &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConsoleLink",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultConsoleLinkName(),
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: "https://" + cheHost,
				Text: DefaultConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  DefaultConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", cheHost, DefaultConsoleLinkImage()),
			},
		},
	}
}

func HasConsolelinkObject() bool {
	resourceList, err := util.GetServerResources()
	if err != nil {
		return false
	}
	for _, res := range resourceList {
		for _, r := range res.APIResources {
			if r.Name == "consolelinks" {
				return true
			}
		}
	}
	return false
}
