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

package imagepuller

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/google/go-cmp/cmp"
	ctrl "sigs.k8s.io/controller-runtime"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
)

var (
	log                           = ctrl.Log.WithName("image-puller")
	kubernetesImagePullerDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(chev1alpha1.KubernetesImagePuller{}, "TypeMeta", "ObjectMeta", "Status"),
	}
)

const (
	resourceName  = "kubernetesimagepullers"
	finalizerName = "kubernetesimagepullers.finalizers.che.eclipse.org"

	defaultConfigMapName    = "k8s-image-puller"
	defaultDeploymentName   = "kubernetes-image-puller"
	defaultImagePullerImage = "quay.io/eclipse/kubernetes-image-puller:next"
)

type ImagePuller struct {
	deploy.Reconcilable
}

func NewImagePuller() *ImagePuller {
	return &ImagePuller{}
}

func (ip *ImagePuller) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.Spec.Components.ImagePuller.Enable {
		if !utils.IsK8SResourceServed(ctx.ClusterAPI.DiscoveryClient, resourceName) {
			errMsg := "Kubernetes Image Puller is not installed, in order to enable the property admin should install the operator first"
			return reconcile.Result{}, false, fmt.Errorf(errMsg)
		}

		if done, err := ip.syncKubernetesImagePuller(ctx); !done {
			return reconcile.Result{Requeue: true}, false, err
		}
	} else {
		if done, err := ip.uninstallImagePuller(ctx); !done {
			return reconcile.Result{Requeue: true}, false, err
		}
	}
	return reconcile.Result{}, true, nil
}

func (ip *ImagePuller) Finalize(ctx *chetypes.DeployContext) bool {
	done, err := ip.uninstallImagePuller(ctx)
	if err != nil {
		log.Error(err, "Failed to uninstall Kubernetes Image Puller")
	}
	return done
}

func (ip *ImagePuller) uninstallImagePuller(ctx *chetypes.DeployContext) (bool, error) {
	// Keep it here for backward compatability
	if err := deploy.DeleteFinalizer(ctx, finalizerName); err != nil {
		return false, err
	}

	if utils.IsK8SResourceServed(ctx.ClusterAPI.DiscoveryClient, resourceName) {
		if done, err := deploy.DeleteByKeyWithClient(
			ctx.ClusterAPI.NonCachingClient,
			types.NamespacedName{
				Namespace: ctx.CheCluster.Namespace,
				Name:      getImagePullerCustomResourceName(ctx)},
			&chev1alpha1.KubernetesImagePuller{},
		); !done {
			return false, err
		}
	}

	return true, nil
}

func (ip *ImagePuller) syncKubernetesImagePuller(ctx *chetypes.DeployContext) (bool, error) {
	imagePuller := &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: chev1alpha1.GroupVersion.String(),
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getImagePullerCustomResourceName(ctx),
			Namespace: ctx.CheCluster.Namespace,
			Labels: map[string]string{
				constants.KubernetesComponentLabelKey: constants.KubernetesImagePullerComponentName,
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
			},
		},
		Spec: *ctx.CheCluster.Spec.Components.ImagePuller.Spec.DeepCopy(),
	}

	// Set default values to avoid syncing object on every loop
	// See https://github.com/che-incubator/kubernetes-image-puller-operator/blob/main/controllers/kubernetesimagepuller_controller.go
	imagePuller.Spec.ConfigMapName = utils.GetValue(imagePuller.Spec.ConfigMapName, defaultConfigMapName)
	imagePuller.Spec.DeploymentName = utils.GetValue(imagePuller.Spec.DeploymentName, defaultDeploymentName)
	imagePuller.Spec.ImagePullerImage = utils.GetValue(imagePuller.Spec.ImagePullerImage, defaultImagePullerImage)
	if strings.TrimSpace(imagePuller.Spec.Images) == "" {
		imagePuller.Spec.Images = getDefaultImages(ctx)
	}

	return deploy.SyncWithClient(ctx.ClusterAPI.NonCachingClient, ctx, imagePuller, kubernetesImagePullerDiffOpts)
}

func getImagePullerCustomResourceName(ctx *chetypes.DeployContext) string {
	return ctx.CheCluster.Name + "-image-puller"
}

func getDefaultImages(ctx *chetypes.DeployContext) string {
	urls := collectUrls(ctx)
	allImages := fetchImages(urls, ctx)
	sortedImages := sortImages(allImages)
	return images2string(sortedImages)
}

func collectUrls(ctx *chetypes.DeployContext) []string {
	urls2fetch := make([]string, 0)

	if ctx.CheCluster.Status.PluginRegistryURL != "" {
		urls2fetch = append(
			urls2fetch,
			fmt.Sprintf(
				"http://%s.%s.svc:8080/v3/%s",
				constants.PluginRegistryName,
				ctx.CheCluster.Namespace,
				"external_images.txt",
			),
		)
	}

	if ctx.CheCluster.Status.DevfileRegistryURL != "" {
		urls2fetch = append(
			urls2fetch,
			fmt.Sprintf(
				"http://%s.%s.svc:8080/%s",
				constants.DevfileRegistryName,
				ctx.CheCluster.Namespace,
				"devfiles/external_images.txt",
			),
		)
	}

	return urls2fetch
}

func fetchImages(urls []string, ctx *chetypes.DeployContext) map[string]bool {
	allImages := make(map[string]bool)

	for _, url := range urls {
		images, err := fetchImagesFromUrl(url, ctx)
		if err != nil {
			log.Error(err, fmt.Sprintf("Failed to fetch images from %s", url))
		} else {
			for image := range images {
				allImages[image] = true
			}
		}
	}

	return allImages
}

func sortImages(images map[string]bool) []string {
	sortedImages := make([]string, len(images))

	i := 0
	for image := range images {
		sortedImages[i] = image
		i++
	}

	sort.Strings(sortedImages)
	return sortedImages
}

func images2string(images []string) string {
	imagesAsString := ""
	for index, image := range images {
		imagesAsString += fmt.Sprintf("image-%d=%s;", index, image)
	}

	return imagesAsString
}

func fetchImagesFromUrl(url string, ctx *chetypes.DeployContext) (map[string]bool, error) {
	images := make(map[string]bool)

	transport := &http.Transport{}
	if ctx.Proxy.HttpProxy != "" {
		deploy.ConfigureProxy(ctx, transport)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   time.Second * 3,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return images, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return images, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		return images, err
	}

	for _, image := range strings.Split(string(data), "\n") {
		image = strings.TrimSpace(image)
		if image != "" {
			images[image] = true
		}
	}

	return images, nil
}
