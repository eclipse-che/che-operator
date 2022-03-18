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
	"strings"
	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	log = ctrl.Log.WithName("image-puller")
)

var imagePullerFinalizerName = "kubernetesimagepullers.finalizers.che.eclipse.org"

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

func (ip *ImagePuller) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	return ReconcileImagePuller(ctx)
}

func (ip *ImagePuller) Finalize(ctx *deploy.DeployContext) bool {
	return DeleteImagePullerOperatorAndFinalizer(ctx)
}

// Reconcile the imagePuller section of the CheCluster CR.  If imagePuller.enable is set to true, install the Kubernetes Image Puller operator and create
// a KubernetesImagePuller CR.  Add a finalizer to the CheCluster CR.  If false, remove the KubernetesImagePuller CR, uninstall the operator, and remove the finalizer.
func ReconcileImagePuller(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	// Determine what server groups the API Server knows about
	foundPackagesAPI, foundOperatorsAPI, _, err := CheckNeededImagePullerApis(ctx)
	if err != nil {
		logrus.Errorf("Error discovering image puller APIs: %v", err)
		return reconcile.Result{}, false, err
	}

	// If the image puller should be installed but the APIServer doesn't know about PackageManifests/Subscriptions, log a warning and requeue
	if ctx.CheCluster.Spec.ImagePuller.Enable && (!foundPackagesAPI || !foundOperatorsAPI) {
		logrus.Infof("Couldn't find Operator Lifecycle Manager types to install the Kubernetes Image Puller Operator.  Please install Operator Lifecycle Manager to install the operator or disable the image puller by setting spec.imagePuller.enable to false.")
		return reconcile.Result{RequeueAfter: time.Second}, false, nil
	}

	if ctx.CheCluster.Spec.ImagePuller.Enable {
		if foundOperatorsAPI && foundPackagesAPI {
			packageManifest, err := GetPackageManifest(ctx)
			if err != nil {
				if errors.IsNotFound(err) {
					logrus.Infof("There is no PackageManifest for the Kubernetes Image Puller Operator.  Install the Operator Lifecycle Manager and the Community Operators Catalog")
					return reconcile.Result{RequeueAfter: time.Second}, false, nil
				}
				logrus.Errorf("Error getting packagemanifest: %v", err)
				return reconcile.Result{}, false, err
			}

			createdOperatorGroup, err := CreateOperatorGroupIfNotFound(ctx)
			if err != nil {
				logrus.Infof("Error creating OperatorGroup: %v", err)
				return reconcile.Result{}, false, err
			}
			if createdOperatorGroup {
				return reconcile.Result{RequeueAfter: time.Second}, false, nil
			}
			createdOperatorSubscription, err := CreateImagePullerSubscription(ctx, packageManifest)
			if err != nil {
				logrus.Infof("Error creating Subscription: %v", err)
				return reconcile.Result{}, false, err
			}
			if createdOperatorSubscription {
				return reconcile.Result{RequeueAfter: time.Second}, false, nil
			}

			// Add the image puller finalizer
			if !HasImagePullerFinalizer(ctx.CheCluster) {
				if err := ReconcileImagePullerFinalizer(ctx); err != nil {
					return reconcile.Result{}, false, err
				}
				return reconcile.Result{RequeueAfter: time.Second}, false, nil
			}
		}

		_, _, foundKubernetesImagePullerAPI, err := CheckNeededImagePullerApis(ctx)
		if err != nil {
			logrus.Errorf("Error discovering image puller APIs: %v", err)
			return reconcile.Result{}, false, err
		}
		// If the KubernetesImagePuller API service exists, attempt to reconcile creation/update
		if foundKubernetesImagePullerAPI {
			// Check KubernetesImagePuller options
			imagePuller := &chev1alpha1.KubernetesImagePuller{}
			err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: ctx.CheCluster.Name + "-image-puller"}, imagePuller)
			if err != nil {
				if errors.IsNotFound(err) {

					// If the image puller spec is empty, set default values, update the CheCluster CR and requeue
					// These assignments are needed because the image puller operator updates the CR with a default configmap and deployment name
					// if none are given.  Without these, che-operator will be stuck in an update loop
					if ctx.CheCluster.IsImagePullerSpecEmpty() {
						logrus.Infof("Updating CheCluster to set KubernetesImagePuller default values")
						_, err := UpdateImagePullerSpecIfEmpty(ctx)
						if err != nil {
							logrus.Errorf("Error updating CheCluster: %v", err)
							return reconcile.Result{}, false, err
						}
						return reconcile.Result{RequeueAfter: time.Second}, false, nil
					}

					if ctx.CheCluster.IsImagePullerImagesEmpty() {
						if err = SetDefaultImages(ctx); err != nil {
							logrus.Error(err)
							return reconcile.Result{}, false, err
						}
					}

					logrus.Infof("Creating KubernetesImagePuller for CheCluster %v", ctx.CheCluster.Name)
					createdImagePuller, err := CreateKubernetesImagePuller(ctx)
					if err != nil {
						logrus.Error("Error creating KubernetesImagePuller: ", err)
						return reconcile.Result{}, false, err
					}
					if createdImagePuller {
						return reconcile.Result{}, false, nil
					}
				}
				logrus.Errorf("Error getting KubernetesImagePuller: %v", err)
				return reconcile.Result{}, false, err
			}

			if err = UpdateDefaultImagesIfNeeded(ctx); err != nil {
				logrus.Error(err)
				return reconcile.Result{}, false, err
			}

			if ctx.CheCluster.Spec.ImagePuller.Spec.DeploymentName == "" {
				ctx.CheCluster.Spec.ImagePuller.Spec.DeploymentName = imagePuller.Spec.DeploymentName
			}
			if ctx.CheCluster.Spec.ImagePuller.Spec.ConfigMapName == "" {
				ctx.CheCluster.Spec.ImagePuller.Spec.ConfigMapName = imagePuller.Spec.ConfigMapName
			}
			if ctx.CheCluster.Spec.ImagePuller.Spec.ImagePullerImage == "" {
				ctx.CheCluster.Spec.ImagePuller.Spec.ImagePullerImage = imagePuller.Spec.ImagePullerImage
			}

			// If ImagePuller specs are different, update the KubernetesImagePuller CR
			if imagePuller.Spec != ctx.CheCluster.Spec.ImagePuller.Spec {
				imagePuller.Spec = ctx.CheCluster.Spec.ImagePuller.Spec
				logrus.Infof("Updating KubernetesImagePuller %v", imagePuller.Name)
				if err = ctx.ClusterAPI.Client.Update(context.TODO(), imagePuller, &client.UpdateOptions{}); err != nil {
					logrus.Errorf("Error updating KubernetesImagePuller: %v", err)
					return reconcile.Result{}, false, err
				}
				return reconcile.Result{RequeueAfter: time.Second}, false, nil
			}
		} else {
			logrus.Infof("Waiting 15 seconds for kubernetesimagepullers.che.eclipse.org API")
			return reconcile.Result{RequeueAfter: 15 * time.Second}, false, nil
		}

	} else {
		if foundOperatorsAPI && foundPackagesAPI {
			done := DeleteImagePullerOperatorAndFinalizer(ctx)
			return reconcile.Result{}, done, nil
		}
	}
	return reconcile.Result{}, true, nil
}

func DeleteImagePullerOperatorAndFinalizer(ctx *deploy.DeployContext) bool {
	done := true

	if _, err := GetImagePullerOperator(ctx); err == nil {
		if _, err := UninstallImagePullerOperator(ctx); err != nil {
			done = false
			logrus.Errorf("Error uninstalling Image Puller: %v", err)
		}
	}

	if HasImagePullerFinalizer(ctx.CheCluster) {
		if err := DeleteImagePullerFinalizer(ctx); err != nil {
			done = false
			logrus.Errorf("Error deleting finalizer: %v", err)
		}
	}

	return done
}

func HasImagePullerFinalizer(instance *orgv1.CheCluster) bool {
	finalizers := instance.ObjectMeta.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == imagePullerFinalizerName {
			return true
		}
	}
	return false
}

func ReconcileImagePullerFinalizer(ctx *deploy.DeployContext) (err error) {
	instance := ctx.CheCluster
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		return deploy.AppendFinalizer(ctx, imagePullerFinalizerName)
	} else {
		if util.ContainsString(instance.ObjectMeta.Finalizers, imagePullerFinalizerName) {
			clusterServiceVersionName := deploy.DefaultKubernetesImagePullerOperatorCSV()
			logrus.Infof("Custom resource %s is being deleted. Deleting ClusterServiceVersion %s first", instance.Name, clusterServiceVersionName)
			clusterServiceVersion := &operatorsv1alpha1.ClusterServiceVersion{}
			err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: instance.Namespace, Name: clusterServiceVersionName}, clusterServiceVersion)
			if err != nil {
				logrus.Errorf("Error getting ClusterServiceVersion: %v", err)
				return err
			}
			if err := ctx.ClusterAPI.Client.Delete(context.TODO(), clusterServiceVersion); err != nil {
				logrus.Errorf("Failed to delete %s ClusterServiceVersion: %s", clusterServiceVersionName, err)
				return err
			}

			return deploy.DeleteFinalizer(ctx, imagePullerFinalizerName)
		}
		return nil
	}
}

func DeleteImagePullerFinalizer(ctx *deploy.DeployContext) (err error) {
	instance := ctx.CheCluster
	instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, imagePullerFinalizerName)
	logrus.Infof("Removing image puller finalizer on %s CR", instance.Name)
	if err := ctx.ClusterAPI.Client.Update(context.Background(), instance); err != nil {
		logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
		return err
	}
	return nil
}

// Returns true if the expected and actual Subscription specs have the same fields during Image Puller
// installation
func SubscriptionsAreEqual(expected *operatorsv1alpha1.Subscription, actual *operatorsv1alpha1.Subscription) bool {
	return expected.Spec.CatalogSource == actual.Spec.CatalogSource &&
		expected.Spec.CatalogSourceNamespace == actual.Spec.CatalogSourceNamespace &&
		expected.Spec.Channel == actual.Spec.Channel &&
		expected.Spec.InstallPlanApproval == actual.Spec.InstallPlanApproval &&
		expected.Spec.Package == actual.Spec.Package
}

// Check if the API server can discover the API groups for packages.operators.coreos.com,
// operators.coreos.com, and che.eclipse.org.
// Returns:
// foundPackagesAPI - true if the server discovers the packages.operators.coreos.com API
// foundOperatorsAPI - true if the server discovers the operators.coreos.com API
// foundKubernetesImagePullerAPI - true if the server discovers the che.eclipse.org API
// error - any error returned by the call to discoveryClient.ServerGroups()
func CheckNeededImagePullerApis(ctx *deploy.DeployContext) (bool, bool, bool, error) {
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

// Search for the kubernetes-imagepuller-operator PackageManifest
func GetPackageManifest(ctx *deploy.DeployContext) (*packagesv1.PackageManifest, error) {
	packageManifest := &packagesv1.PackageManifest{}
	err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, packageManifest)
	if err != nil {
		return packageManifest, err
	}
	return packageManifest, nil
}

// Create an OperatorGroup in the CheCluster namespace if it does not exist.  Returns true if the
// OperatorGroup was created, and any error returned during the List and Create operation
func CreateOperatorGroupIfNotFound(ctx *deploy.DeployContext) (bool, error) {
	operatorGroupList := &operatorsv1.OperatorGroupList{}
	err := ctx.ClusterAPI.NonCachingClient.List(context.TODO(), operatorGroupList, &client.ListOptions{Namespace: ctx.CheCluster.Namespace})
	if err != nil {
		return false, err
	}

	if len(operatorGroupList.Items) == 0 {
		operatorGroup := &operatorsv1.OperatorGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-imagepuller-operator",
				Namespace: ctx.CheCluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(ctx.CheCluster, ctx.CheCluster.GroupVersionKind()),
				},
			},
			Spec: operatorsv1.OperatorGroupSpec{
				TargetNamespaces: []string{
					ctx.CheCluster.Namespace,
				},
			},
		}
		logrus.Infof("Creating kubernetes image puller OperatorGroup")
		if err = ctx.ClusterAPI.NonCachingClient.Create(context.TODO(), operatorGroup, &client.CreateOptions{}); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func CreateImagePullerSubscription(ctx *deploy.DeployContext, packageManifest *packagesv1.PackageManifest) (bool, error) {
	imagePullerOperatorSubscription := &operatorsv1alpha1.Subscription{}
	err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{
		Name:      "kubernetes-imagepuller-operator",
		Namespace: ctx.CheCluster.Namespace,
	}, imagePullerOperatorSubscription)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Info("Creating kubernetes image puller operator Subscription")
			err = ctx.ClusterAPI.NonCachingClient.Create(context.TODO(), GetExpectedSubscription(ctx, packageManifest), &client.CreateOptions{})
			if err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func GetExpectedSubscription(ctx *deploy.DeployContext, packageManifest *packagesv1.PackageManifest) *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: ctx.CheCluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ctx.CheCluster, ctx.CheCluster.GroupVersionKind()),
			},
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          packageManifest.Status.CatalogSource,
			CatalogSourceNamespace: packageManifest.Status.CatalogSourceNamespace,
			Channel:                packageManifest.Status.DefaultChannel,
			InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
			Package:                "kubernetes-imagepuller-operator",
		},
	}
}

func GetActualSubscription(ctx *deploy.DeployContext) (*operatorsv1alpha1.Subscription, error) {
	actual := &operatorsv1alpha1.Subscription{}
	err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, actual)
	if err != nil {
		return nil, err
	}
	return actual, nil
}

// Update the CheCluster ImagePuller spec if the default values are not set
// returns the updated spec and an error during update
func UpdateImagePullerSpecIfEmpty(ctx *deploy.DeployContext) (orgv1.CheClusterSpecImagePuller, error) {
	if ctx.CheCluster.Spec.ImagePuller.Spec.DeploymentName == "" {
		ctx.CheCluster.Spec.ImagePuller.Spec.DeploymentName = "kubernetes-image-puller"
	}
	if ctx.CheCluster.Spec.ImagePuller.Spec.ConfigMapName == "" {
		ctx.CheCluster.Spec.ImagePuller.Spec.ConfigMapName = "k8s-image-puller"
	}
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster, &client.UpdateOptions{})
	if err != nil {
		return ctx.CheCluster.Spec.ImagePuller, err
	}
	return ctx.CheCluster.Spec.ImagePuller, nil
}

func SetDefaultImages(ctx *deploy.DeployContext) error {
	defaultImages := GetDefaultImages()
	if len(defaultImages) == 0 {
		return nil
	}
	return SetImages(ctx, defaultImages)
}

// ImageSliceToString returns a string representation of the provided image slice, suitable for the
// imagePuller.spec.images field
func ImageSliceToString(imageSlice []ImageAndName) string {
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
func StringToImageSlice(imagesString string) []ImageAndName {
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

// GetDefaultImages returns the current default images from the environment variables
func GetDefaultImages() []ImageAndName {
	images := []ImageAndName{}
	imagePatterns := [...]string{
		"^RELATED_IMAGE_.*_theia.*",
		"^RELATED_IMAGE_.*_machine(_)?exec(_.*)?_plugin_registry_image.*",
		"^RELATED_IMAGE_.*_kubernetes(_.*)?_plugin_registry_image.*",
		"^RELATED_IMAGE_.*_openshift(_.*)?_plugin_registry_image.*",
		"^RELATED_IMAGE_.*_cpp(_.*)?_devfile_registry_image.*",
		"^RELATED_IMAGE_.*_dotnet(_.*)?_devfile_registry_image.*",
		"^RELATED_IMAGE_.*_golang(_.*)?_devfile_registry_image.*",
		"^RELATED_IMAGE_.*_php(_.*)?_devfile_registry_image.*",
		"^RELATED_IMAGE_.*_java.{1,2}(_maven)?_devfile_registry_image.*",
	}
	for _, pattern := range imagePatterns {
		matches := util.GetEnvByRegExp(pattern)
		for _, match := range matches {
			match.Name = match.Name[len("RELATED_IMAGE_"):]
			images = append(images, ImageAndName{Name: match.Name, Image: match.Value})
		}
	}
	return images
}

// Convert input string to RFC 1123 format ([a-z0-9]([-a-z0-9]*[a-z0-9])?) max 63 characters, if possible
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

func CreateKubernetesImagePuller(ctx *deploy.DeployContext) (bool, error) {
	imagePuller := GetExpectedKubernetesImagePuller(ctx)
	err := ctx.ClusterAPI.Client.Create(context.TODO(), imagePuller, &client.CreateOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetExpectedKubernetesImagePuller(ctx *deploy.DeployContext) *chev1alpha1.KubernetesImagePuller {
	return &chev1alpha1.KubernetesImagePuller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ctx.CheCluster.Name + "-image-puller",
			Namespace: ctx.CheCluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ctx.CheCluster, ctx.CheCluster.GroupVersionKind()),
			},
			Labels: map[string]string{
				"app.kubernetes.io/part-of": deploy.CheEclipseOrg,
				"app":                       deploy.DefaultCheFlavor(ctx.CheCluster),
				"component":                 "kubernetes-image-puller",
			},
		},
		Spec: ctx.CheCluster.Spec.ImagePuller.Spec,
	}
}

// UpdateDefaultImagesIfNeeded updates the default images from `spec.images` if needed
func UpdateDefaultImagesIfNeeded(ctx *deploy.DeployContext) error {
	specImages := StringToImageSlice(ctx.CheCluster.Spec.ImagePuller.Spec.Images)
	defaultImages := GetDefaultImages()
	if UpdateSpecImages(specImages, defaultImages) {
		return SetImages(ctx, specImages)
	}
	return nil
}

// UpdateSpecImages returns true if the default images from `spec.images` were updated
// with new defaults
//
// specImages contains the images in `spec.images`
// defaultImages contains the current default images from the environment variables
func UpdateSpecImages(specImages []ImageAndName, defaultImages []ImageAndName) bool {
	match := false
	for i, specImage := range specImages {
		specImageName, specImageTag := util.GetImageNameAndTag(specImage.Image)
		for _, defaultImage := range defaultImages {
			defaultImageName, defaultImageTag := util.GetImageNameAndTag(defaultImage.Image)
			// if the image tags are different for this image, then update
			if defaultImageName == specImageName && defaultImageTag != specImageTag {
				match = true
				specImages[i].Image = defaultImage.Image
				break
			}
		}
	}
	return match
}

// SetImages sets the provided images to the imagePuller spec
func SetImages(ctx *deploy.DeployContext, images []ImageAndName) error {
	imagesStr := ImageSliceToString(images)
	ctx.CheCluster.Spec.ImagePuller.Spec.Images = imagesStr
	return deploy.UpdateCheCRSpec(ctx, "Kubernetes Image Puller images", imagesStr)
}

// Uninstall the CSV, OperatorGroup, Subscription, KubernetesImagePuller, and update the CheCluster to remove
// the image puller spec.  Returns true if the CheCluster was updated
func UninstallImagePullerOperator(ctx *deploy.DeployContext) (bool, error) {
	updated := false
	_, hasOperatorsAPIs, hasImagePullerAPIs, err := CheckNeededImagePullerApis(ctx)
	if err != nil {
		return updated, err
	}

	if hasImagePullerAPIs {
		// Delete the KubernetesImagePuller
		imagePuller, err := GetImagePullerOperator(ctx)
		if err != nil && !errors.IsNotFound(err) {
			return updated, err
		}
		if imagePuller.Name != "" {
			logrus.Infof("Deleting KubernetesImagePuller %v", imagePuller.Name)
			if err = ctx.ClusterAPI.Client.Delete(context.TODO(), imagePuller, &client.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return updated, err
			}
		}
	}

	if hasOperatorsAPIs {
		// Delete the ClusterServiceVersion
		csv := &operatorsv1alpha1.ClusterServiceVersion{}
		err = ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: deploy.DefaultKubernetesImagePullerOperatorCSV()}, csv)
		if err != nil && !errors.IsNotFound(err) {
			return updated, err
		}

		if csv.Name != "" {
			logrus.Infof("Deleting ClusterServiceVersion %v", csv.Name)
			err := ctx.ClusterAPI.NonCachingClient.Delete(context.TODO(), csv, &client.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return updated, err
			}
		}

		// Delete the Subscription
		subscription := &operatorsv1alpha1.Subscription{}
		err = ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, subscription)
		if err != nil && !errors.IsNotFound(err) {
			return updated, err
		}

		if subscription.Name != "" {
			logrus.Infof("Deleting Subscription %v", subscription.Name)
			err := ctx.ClusterAPI.NonCachingClient.Delete(context.TODO(), subscription, &client.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return updated, err
			}
		}
		// Delete the OperatorGroup if it was created
		operatorGroup := &operatorsv1.OperatorGroup{}
		err = ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, operatorGroup)
		if err != nil && !errors.IsNotFound(err) {
			return updated, err
		}

		if operatorGroup.Name != "" {
			logrus.Infof("Deleting OperatorGroup %v", operatorGroup.Name)
			err := ctx.ClusterAPI.NonCachingClient.Delete(context.TODO(), operatorGroup, &client.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return updated, err
			}
		}
	}

	// Update CR to remove imagePullerSpec
	if (ctx.CheCluster.Spec.ImagePuller.Enable || ctx.CheCluster.Spec.ImagePuller.Spec != (chev1alpha1.KubernetesImagePullerSpec{})) &&
		ctx.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		ctx.CheCluster.Spec.ImagePuller.Spec = chev1alpha1.KubernetesImagePullerSpec{}
		logrus.Infof("Updating CheCluster %v to remove image puller spec", ctx.CheCluster.Name)
		err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster, &client.UpdateOptions{})
		if err != nil {
			return updated, err
		}
		updated = true
	}
	return updated, nil
}

// GetImagePullerOperator returns the current kubernetes-imagepuller-operator if exists
func GetImagePullerOperator(ctx *deploy.DeployContext) (*chev1alpha1.KubernetesImagePuller, error) {
	imagePuller := &chev1alpha1.KubernetesImagePuller{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: ctx.CheCluster.Name + "-image-puller"}, imagePuller)
	if err != nil {
		return nil, err
	}
	return imagePuller, nil
}
