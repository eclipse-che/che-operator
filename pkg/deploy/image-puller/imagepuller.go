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
	"context"
	goerror "errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	diffOpts = cmp.Options{
		cmpopts.IgnoreFields(chev1alpha1.KubernetesImagePuller{}, "TypeMeta", "ObjectMeta"),
	}
)

const (
	subscriptionName         = "kubernetes-imagepuller-operator"
	operatorGroupName        = "kubernetes-imagepuller-operator"
	packageName              = "kubernetes-imagepuller-operator"
	componentName            = "kubernetes-image-puller"
	defaultConfigMapName     = "k8s-image-puller"
	defaultDeploymentName    = "kubernetes-image-puller"
	imagePullerFinalizerName = "kubernetesimagepullers.finalizers.che.eclipse.org"
)

// ImageAndName represents an image coupled with an image name.
type ImageAndName struct {
	Name  string // image name (ex. my-image)
	Image string // image (ex. quay.io/test/abc)
}

type ImagePuller struct {
	deploy.Reconcilable
}

func NewImagePuller() *ImagePuller {
	return &ImagePuller{}
}

func (ip *ImagePuller) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.Spec.Components.ImagePuller.Enable {
		if foundPackagesAPI, foundOperatorsAPI, _, err := ip.discoverImagePullerApis(ctx); !foundPackagesAPI || !foundOperatorsAPI {
			if err != nil {
				return reconcile.Result{}, false, err
			}
			errorMsg := "couldn't find Operator Lifecycle Manager types to install the Kubernetes Image Puller Operator. Please install Operator Lifecycle Manager to install the operator or disable the image puller by setting `spec.imagePuller.enable` to false"
			return reconcile.Result{RequeueAfter: time.Second}, false, fmt.Errorf(errorMsg)
		}

		if done, err := ip.syncOperatorGroup(ctx); !done {
			return reconcile.Result{}, false, err
		}

		if done, err := ip.syncSubscription(ctx); !done {
			return reconcile.Result{}, false, err
		}

		if done, err := ip.syncDefaultImages(ctx); !done {
			return reconcile.Result{}, false, err
		}

		if err := deploy.AppendFinalizer(ctx, imagePullerFinalizerName); err != nil {
			return reconcile.Result{}, false, err
		}

		// Wait for KubernetesImagePuller API
		if _, _, foundKubernetesImagePullerAPI, err := ip.discoverImagePullerApis(ctx); !foundKubernetesImagePullerAPI {
			if err != nil {
				return reconcile.Result{}, false, err
			}
			logrus.Infof("Waiting 15 seconds for kubernetesimagepullers.che.eclipse.org API")
			return reconcile.Result{RequeueAfter: 15 * time.Second}, false, nil
		}

		if done, err := ip.syncKubernetesImagePuller(ctx); !done {
			return reconcile.Result{}, false, err
		}
	} else {
		if done, err := ip.uninstallImagePullerOperator(ctx); done {
			return reconcile.Result{}, false, err
		}
	}
	return reconcile.Result{}, true, nil
}

func (ip *ImagePuller) Finalize(ctx *chetypes.DeployContext) bool {
	done, err := ip.uninstallImagePullerOperator(ctx)
	if err != nil {
		log.Error(err, "Failed to uninstall KubernetesImagePuller")
	}
	return done
}

// Uninstall the CSV, OperatorGroup, Subscription, KubernetesImagePuller, and update the CheCluster to remove
// the image puller spec.  Returns true if the CheCluster was updated
func (ip *ImagePuller) uninstallImagePullerOperator(ctx *chetypes.DeployContext) (bool, error) {
	_, foundOperatorsAPI, foundKubernetesImagePullerAPI, err := ip.discoverImagePullerApis(ctx)
	if err != nil {
		return false, err
	}

	if foundKubernetesImagePullerAPI {
		if done, err := deploy.DeleteByKeyWithClient(
			ctx.ClusterAPI.NonCachingClient,
			types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: getImagePullerOperatorName(ctx)},
			&chev1alpha1.KubernetesImagePuller{},
		); !done {
			return false, err
		}
	}

	if foundOperatorsAPI {
		// Delete the Subscription and ClusterServiceVersion
		subscription := &operatorsv1alpha1.Subscription{}
		if exists, err := deploy.GetWithClient(ctx.ClusterAPI.NonCachingClient, types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: subscriptionName}, subscription); err != nil {
			return false, err
		} else if exists {
			if subscription.Status.InstalledCSV != "" {
				if done, err := deploy.DeleteByKeyWithClient(
					ctx.ClusterAPI.NonCachingClient,
					types.NamespacedName{
						Namespace: ctx.CheCluster.Namespace,
						Name:      subscription.Status.InstalledCSV,
					},
					&operatorsv1alpha1.ClusterServiceVersion{}); !done {
					return false, err
				}
			}

			if done, err := deploy.DeleteByKeyWithClient(
				ctx.ClusterAPI.NonCachingClient,
				types.NamespacedName{
					Namespace: ctx.CheCluster.Namespace,
					Name:      subscriptionName,
				},
				&operatorsv1alpha1.Subscription{}); !done {
				return false, err
			}
		}

		// Delete the OperatorGroup
		if done, err := deploy.DeleteByKeyWithClient(
			ctx.ClusterAPI.NonCachingClient,
			types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: operatorGroupName},
			&operatorsv1.OperatorGroup{},
		); !done {
			return false, err
		}
	}

	if err := deploy.DeleteFinalizer(ctx, imagePullerFinalizerName); err != nil {
		return false, err
	}

	return true, nil
}

// CheckNeededImagePullerApis check if the API server can discover the API groups
// for packages.operators.coreos.com, operators.coreos.com, and che.eclipse.org.
// Returns:
// foundPackagesAPI - true if the server discovers the packages.operators.coreos.com API
// foundOperatorsAPI - true if the server discovers the operators.coreos.com API
// foundKubernetesImagePullerAPI - true if the server discovers the che.eclipse.org API
// error - any error returned by the call to discoveryClient.ServerGroups()
func (ip *ImagePuller) discoverImagePullerApis(ctx *chetypes.DeployContext) (bool, bool, bool, error) {
	groupList, resourcesList, err := ctx.ClusterAPI.DiscoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false, false, false, err
	}
	foundPackagesAPI := false
	foundOperatorsAPI := false
	foundKubernetesImagePullerAPI := false
	for _, group := range groupList {
		if group.Name == packagesv1.SchemeGroupVersion.Group {
			foundPackagesAPI = true
		}
		if group.Name == operatorsv1alpha1.SchemeGroupVersion.Group {
			foundOperatorsAPI = true
		}
	}

	for _, l := range resourcesList {
		for _, r := range l.APIResources {
			if l.GroupVersion == chev1alpha1.SchemeBuilder.GroupVersion.String() && r.Kind == "KubernetesImagePuller" {
				foundKubernetesImagePullerAPI = true
			}
		}
	}
	return foundPackagesAPI, foundOperatorsAPI, foundKubernetesImagePullerAPI, nil
}

func (ip *ImagePuller) syncDefaultImages(ctx *chetypes.DeployContext) (bool, error) {
	defaultImages := getDefaultImages()
	specImages := stringToImageSlice(ctx.CheCluster.Spec.Components.ImagePuller.Spec.Images)

	if len(specImages) == 0 {
		if len(defaultImages) != 0 {
			err := setImages(ctx, defaultImages)
			return err == nil, err
		}
	} else {
		if updateSpecImages(specImages, defaultImages) {
			err := setImages(ctx, specImages)
			return err == nil, err
		}
	}

	return true, nil
}

// UpdateSpecImages returns true if the default images from `spec.images` were updated
// with new defaults.
func updateSpecImages(specImages []ImageAndName, defaultImages []ImageAndName) bool {
	updated := false
	for i, specImage := range specImages {
		specImageName, specImageTag := utils.GetImageNameAndTag(specImage.Image)
		for _, defaultImage := range defaultImages {
			defaultImageName, defaultImageTag := utils.GetImageNameAndTag(defaultImage.Image)
			// if the image tags are different for this image, then update
			if defaultImageName == specImageName && defaultImageTag != specImageTag {
				updated = true
				specImages[i].Image = defaultImage.Image
				break
			}
		}
	}

	return updated
}

// SetImages sets the provided images to the imagePuller spec
func setImages(ctx *chetypes.DeployContext, images []ImageAndName) error {
	imagesStr := imageSliceToString(images)
	ctx.CheCluster.Spec.Components.ImagePuller.Spec.Images = imagesStr
	return deploy.UpdateCheCRSpec(ctx, "Kubernetes Image Puller images", imagesStr)
}

// GetDefaultImages returns the current default images from the environment variables
func getDefaultImages() []ImageAndName {
	images := []ImageAndName{}
	for _, pattern := range defaultImagePatterns {
		matches := utils.GetEnvsByRegExp(pattern)
		for _, match := range matches {
			match.Name = match.Name[len("RELATED_IMAGE_"):]
			images = append(images, ImageAndName{Name: match.Name, Image: match.Value})
		}
	}
	return images
}

func (ip *ImagePuller) syncOperatorGroup(ctx *chetypes.DeployContext) (bool, error) {
	operatorGroupList := &operatorsv1.OperatorGroupList{}
	if err := ctx.ClusterAPI.NonCachingClient.List(context.TODO(), operatorGroupList, &client.ListOptions{Namespace: ctx.CheCluster.Namespace}); err != nil {
		return false, err
	}

	if len(operatorGroupList.Items) != 0 {
		return true, nil
	}

	operatorGroup := &operatorsv1.OperatorGroup{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OperatorGroup",
			APIVersion: operatorsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorGroupName,
			Namespace: ctx.CheCluster.Namespace,
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: componentName,
			},
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{},
		},
	}

	_, err := deploy.CreateIfNotExistsWithClient(ctx.ClusterAPI.NonCachingClient, ctx, operatorGroup)
	return err == nil, err
}

func (ip *ImagePuller) syncSubscription(ctx *chetypes.DeployContext) (bool, error) {
	packageManifest := &packagesv1.PackageManifest{}
	if exists, err := deploy.GetWithClient(ctx.ClusterAPI.NonCachingClient, types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: packageName}, packageManifest); !exists {
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("there is no PackageManifest for the Kubernetes Image Puller Operator. Install the Operator Lifecycle Manager and the Community Operators Catalog")
	}

	subscription := &operatorsv1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Subscription",
			APIVersion: operatorsv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      subscriptionName,
			Namespace: ctx.CheCluster.Namespace,
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: componentName,
			},
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          packageManifest.Status.CatalogSource,
			CatalogSourceNamespace: packageManifest.Status.CatalogSourceNamespace,
			Channel:                packageManifest.Status.DefaultChannel,
			InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
			Package:                packageName,
		},
	}

	_, err := deploy.CreateIfNotExistsWithClient(ctx.ClusterAPI.NonCachingClient, ctx, subscription)
	return err == nil, err
}

// imageSliceToString returns a string representation of the provided image slice, suitable for the
// imagePuller.spec.images field
func imageSliceToString(imageSlice []ImageAndName) string {
	var err error
	imagesString := ""
	for _, image := range imageSlice {
		image.Name, err = ConvertToRFC1123(image.Name)
		if err != nil {
			continue
		}
		imagesString += image.Name + "=" + image.Image + ";"
	}
	return imagesString
}

// StringToImageSlice returns a slice of ImageAndName structs from the provided semi-colon seperated string
// of key value pairs
func stringToImageSlice(imagesString string) []ImageAndName {
	currentImages := strings.Split(imagesString, ";")
	for i, image := range currentImages {
		currentImages[i] = strings.TrimSpace(image)
	}
	// Remove the last element, if empty
	if currentImages[len(currentImages)-1] == "" {
		currentImages = currentImages[:len(currentImages)-1]
	}

	images := []ImageAndName{}
	for _, image := range currentImages {
		nameAndImage := strings.Split(image, "=")
		if len(nameAndImage) != 2 {
			logrus.Warnf("Malformed image name/tag: %s. Ignoring.", image)
			continue
		}
		images = append(images, ImageAndName{Name: nameAndImage[0], Image: nameAndImage[1]})
	}

	return images
}

// ConvertToRFC1123 converts input string to RFC 1123 format ([a-z0-9]([-a-z0-9]*[a-z0-9])?) max 63 characters, if possible
func ConvertToRFC1123(str string) (string, error) {
	result := strings.ToLower(str)
	if len(str) > validation.DNS1123LabelMaxLength {
		result = result[:validation.DNS1123LabelMaxLength]
	}

	// Remove illegal trailing characters
	i := len(result) - 1
	for i >= 0 && !IsRFC1123Char(result[i]) {
		i -= 1
	}
	result = result[:i+1]

	result = strings.ReplaceAll(result, "_", "-")

	if errs := validation.IsDNS1123Label(result); len(errs) > 0 {
		return "", goerror.New("Cannot convert the following string to RFC 1123 format: " + str)
	}
	return result, nil
}

func IsRFC1123Char(ch byte) bool {
	errs := validation.IsDNS1123Label(string(ch))
	return len(errs) == 0
}

func (ip *ImagePuller) syncKubernetesImagePuller(ctx *chetypes.DeployContext) (bool, error) {
	imagePuller := &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: chev1alpha1.GroupVersion.String(),
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getImagePullerOperatorName(ctx),
			Namespace: ctx.CheCluster.Namespace,
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: componentName,
			},
		},
		Spec: ctx.CheCluster.Spec.Components.ImagePuller.Spec,
	}
	imagePuller.Spec.ConfigMapName = utils.GetValue(imagePuller.Spec.ConfigMapName, defaultConfigMapName)
	imagePuller.Spec.DeploymentName = utils.GetValue(imagePuller.Spec.DeploymentName, defaultDeploymentName)

	return deploy.SyncWithClient(ctx.ClusterAPI.NonCachingClient, ctx, imagePuller, diffOpts)
}

func getImagePullerOperatorName(ctx *chetypes.DeployContext) string {
	return ctx.CheCluster.Name + "-image-puller"
}
