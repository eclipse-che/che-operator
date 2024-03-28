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

	"k8s.io/apimachinery/pkg/util/validation"

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
	urls := collectRegistriesUrls(ctx)
	allImages := fetchImagesFromRegistries(urls, ctx)

	// having them sorted, prevents from constant changing CR spec
	sortedImages := sortImages(allImages)
	return convertToSpecField(sortedImages)
}

func collectRegistriesUrls(ctx *chetypes.DeployContext) []string {
	urls := make([]string, 0)

	if ctx.CheCluster.Status.PluginRegistryURL != "" {
		urls = append(
			urls,
			fmt.Sprintf(
				"http://%s.%s.svc:8080/v3/%s",
				constants.PluginRegistryName,
				ctx.CheCluster.Namespace,
				"external_images.txt",
			),
		)
	}

	if ctx.CheCluster.Status.DevfileRegistryURL != "" {
		urls = append(
			urls,
			fmt.Sprintf(
				"http://%s.%s.svc:8080/%s",
				constants.DevfileRegistryName,
				ctx.CheCluster.Namespace,
				"devfiles/external_images.txt",
			),
		)
	}

	return urls
}

func fetchImagesFromRegistries(urls []string, ctx *chetypes.DeployContext) map[string]bool {
	// return as map to make the list unique
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

func convertToSpecField(images []string) string {
	specField := ""
	for index, image := range images {
		imageEntries := strings.Split(image, "/")
		name, err := convertToRFC1123(imageEntries[len(imageEntries)-1])
		if err != nil {
			name = fmt.Sprintf("image-%d", index)
		}

		// Adding index make the name unique
		specField += fmt.Sprintf("%s-%d=%s;", name, index, image)
	}

	return specField
}

func fetchImagesFromUrl(url string, ctx *chetypes.DeployContext) (map[string]bool, error) {
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
		return map[string]bool{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return map[string]bool{}, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]bool{}, err
	}

	images := make(map[string]bool)
	for _, image := range strings.Split(string(data), "\n") {
		image = strings.TrimSpace(image)
		if image != "" {
			images[image] = true
		}
	}

	if err = resp.Body.Close(); err != nil {
		log.Error(err, "Failed to close a body response")
	}

	return images, nil
}

// convertToRFC1123 converts input string to RFC 1123 format ([a-z0-9]([-a-z0-9]*[a-z0-9])?) max 63 characters, if possible
func convertToRFC1123(str string) (string, error) {
	result := strings.ToLower(str)
	if len(str) > validation.DNS1123LabelMaxLength {
		result = result[:validation.DNS1123LabelMaxLength]
	}

	// Remove illegal trailing characters
	i := len(result) - 1
	for i >= 0 && !isRFC1123Char(result[i]) {
		i -= 1
	}
	result = result[:i+1]

	result = strings.ReplaceAll(result, "_", "-")

	if errs := validation.IsDNS1123Label(result); len(errs) > 0 {
		return "", fmt.Errorf("cannot convert the following string to RFC 1123 format: %s", str)
	}
	return result, nil
}

func isRFC1123Char(ch byte) bool {
	errs := validation.IsDNS1123Label(string(ch))
	return len(errs) == 0
}
