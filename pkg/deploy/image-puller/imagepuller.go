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
package imagepuller

import (
	goerror "errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/google/go-cmp/cmp"
	ctrl "sigs.k8s.io/controller-runtime"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	log                  = ctrl.Log.WithName("image-puller")
	defaultImagePatterns = [...]string{
		"^RELATED_IMAGE_.*_theia.*",
		"^RELATED_IMAGE_.*_code.*",
		"^RELATED_IMAGE_.*_idea.*",
		"^RELATED_IMAGE_.*_machine(_)?exec(_.*)?_plugin_registry_image.*",
		"^RELATED_IMAGE_.*_kubernetes(_.*)?_plugin_registry_image.*",
		"^RELATED_IMAGE_.*_openshift(_.*)?_plugin_registry_image.*",
		"^RELATED_IMAGE_universal(_)?developer(_)?image(_.*)?_devfile_registry_image.*",
	}
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

type Images2Pull = map[string]string

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
			return reconcile.Result{}, false, err
		}
	} else {
		if done, err := ip.uninstallImagePuller(ctx); !done {
			return reconcile.Result{}, false, err
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
	imagePuller.Spec.Images = utils.GetValue(imagePuller.Spec.Images, getDefaultImages())

	return deploy.SyncWithClient(ctx.ClusterAPI.NonCachingClient, ctx, imagePuller, kubernetesImagePullerDiffOpts)
}

func getImagePullerCustomResourceName(ctx *chetypes.DeployContext) string {
	return ctx.CheCluster.Name + "-image-puller"
}

// imagesToString returns a string representation of the provided image slice,
// suitable for the imagePuller.spec.images field
func imagesToString(images Images2Pull) string {
	imageNames := make([]string, 0, len(images))
	for k := range images {
		imageNames = append(imageNames, k)
	}
	sort.Strings(imageNames)

	imagesAsString := ""
	for _, imageName := range imageNames {
		if name, err := convertToRFC1123(imageName); err == nil {
			imagesAsString += name + "=" + images[imageName] + ";"
		}
	}
	return imagesAsString
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
		return "", goerror.New("Cannot convert the following string to RFC 1123 format: " + str)
	}
	return result, nil
}

func isRFC1123Char(ch byte) bool {
	errs := validation.IsDNS1123Label(string(ch))
	return len(errs) == 0
}

// GetDefaultImages returns the current default images from the environment variables
func getDefaultImages() string {
	images := map[string]string{}
	for _, pattern := range defaultImagePatterns {
		matches := utils.GetGetArchitectureDependentEnvsByRegExp(pattern)
		sort.SliceStable(matches, func(i, j int) bool {
			return strings.Compare(matches[i].Name, matches[j].Name) < 0
		})

		for _, match := range matches {
			images[match.Name[len("RELATED_IMAGE_"):]] = match.Value
		}
	}
	return imagesToString(images)
}
