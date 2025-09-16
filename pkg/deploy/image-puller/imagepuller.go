//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
	"errors"
	"fmt"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/google/go-cmp/cmp"
	ctrl "sigs.k8s.io/controller-runtime"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
)

var (
	logger                        = ctrl.Log.WithName("image-puller")
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

	externalImagesFilePath = "/tmp/external_images.txt"
)

type ImagePuller struct {
	deploy.Reconcilable
	imageProvider DefaultImagesProvider
}

func NewImagePuller() *ImagePuller {
	return &ImagePuller{
		imageProvider: NewDashboardApiDefaultImagesProvider(),
	}
}

func (ip *ImagePuller) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	defaultImages, err := ip.imageProvider.get(ctx.CheCluster.Namespace)
	if err != nil {
		logger.Error(err, "Failed to get default images", "error", err)

		// Don't block the reconciliation if we can't get the default images
		return reconcile.Result{}, true, nil
	}

	// Always fetch and persist the default images before actual sync.
	// The purpose is to ability read them from the file on demand by admin (should be documented)
	err = ip.imageProvider.persist(defaultImages, externalImagesFilePath)
	if err != nil {
		logger.Error(err, "Failed to save default images", "error", err)

		// Don't block the reconciliation if we can't save the default images on FS
		return reconcile.Result{}, true, nil
	}

	if ctx.CheCluster.Spec.Components.ImagePuller.Enable {
		if !utils.IsK8SResourceServed(ctx.ClusterAPI.DiscoveryClient, resourceName) {
			errMsg := "Kubernetes Image Puller is not installed, in order to enable the property admin should install the operator first"
			return reconcile.Result{}, false, errors.New(errMsg)
		}

		if done, err := ip.syncKubernetesImagePuller(defaultImages, ctx); !done {
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
		logger.Error(err, "Failed to uninstall Kubernetes Image Puller")
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

func (ip *ImagePuller) syncKubernetesImagePuller(defaultImages []string, ctx *chetypes.DeployContext) (bool, error) {
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
		imagePuller.Spec.Images = convertToSpecField(defaultImages)
	}

	return deploy.SyncForClient(ctx.ClusterAPI.NonCachingClient, ctx, imagePuller, kubernetesImagePullerDiffOpts)
}

func getImagePullerCustomResourceName(ctx *chetypes.DeployContext) string {
	return ctx.CheCluster.Name + "-image-puller"
}

func convertToSpecField(images []string) string {
	specField := ""
	for index, image := range images {
		imageName, _ := utils.GetImageNameAndTag(image)
		imageNameEntries := strings.Split(imageName, "/")
		name, err := convertToRFC1123(imageNameEntries[len(imageNameEntries)-1])
		if err != nil {
			name = "image"
		}

		// Adding index make the name unique
		specField += fmt.Sprintf("%s-%d=%s;", name, index, image)
	}

	return specField
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
