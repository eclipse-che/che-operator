package deploy

import (
	"context"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis/che/v1alpha1"
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
func CheckNeededImagePullerApis(ctx *DeployContext) (bool, bool, bool, error) {
	groupList, err := ctx.ClusterAPI.DiscoveryClient.ServerGroups()
	if err != nil {
		return false, false, false, err
	}
	groups := groupList.Groups
	foundPackagesAPI := false
	foundOperatorsAPI := false
	foundKubernetesImagePullerAPI := false
	for _, group := range groups {
		if group.Name == packagesv1.SchemeGroupVersion.Group {
			foundPackagesAPI = true
		}
		if group.Name == operatorsv1alpha1.SchemeGroupVersion.Group {
			foundOperatorsAPI = true
		}
		if group.Name == chev1alpha1.SchemeGroupVersion.Group {
			foundKubernetesImagePullerAPI = true
		}
	}
	return foundPackagesAPI, foundOperatorsAPI, foundKubernetesImagePullerAPI, nil
}

// Search for the kubernetes-imagepuller-operator PackageManifest
func GetPackageManifest(ctx *DeployContext) (*packagesv1.PackageManifest, error) {
	packageManifest := &packagesv1.PackageManifest{}
	err := ctx.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, packageManifest)
	if err != nil {
		return packageManifest, err
	}
	return packageManifest, nil
}

// Create an OperatorGroup in the CheCluster namespace if it does not exist.  Returns true if the
// OperatorGroup was created, and any error returned during the List and Create operation
func CreateOperatorGroupIfNotFound(ctx *DeployContext) (bool, error) {
	operatorGroupList := &operatorsv1.OperatorGroupList{}
	err := ctx.ClusterAPI.NonCachedClient.List(context.TODO(), operatorGroupList, &client.ListOptions{Namespace: ctx.CheCluster.Namespace})
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
		if err = ctx.ClusterAPI.NonCachedClient.Create(context.TODO(), operatorGroup, &client.CreateOptions{}); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func CreateImagePullerSubscription(ctx *DeployContext, packageManifest *packagesv1.PackageManifest) (bool, error) {
	imagePullerOperatorSubscription := &operatorsv1alpha1.Subscription{}
	err := ctx.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{
		Name:      "kubernetes-imagepuller-operator",
		Namespace: ctx.CheCluster.Namespace,
	}, imagePullerOperatorSubscription)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Info("Creating kubernetes image puller operator Subscription")
			err = ctx.ClusterAPI.NonCachedClient.Create(context.TODO(), GetExpectedSubscription(ctx, packageManifest), &client.CreateOptions{})
			if err != nil {
				return false, err
			}
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func GetExpectedSubscription(ctx *DeployContext, packageManifest *packagesv1.PackageManifest) *operatorsv1alpha1.Subscription {
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

func CompareExpectedSubscription(ctx *DeployContext, packageManifest *packagesv1.PackageManifest) (bool, error) {
	expected := GetExpectedSubscription(ctx, packageManifest)
	actual := &operatorsv1alpha1.Subscription{}
	err := ctx.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, actual)
	if err != nil {
		return false, err
	}
	return SubscriptionsAreEqual(expected, actual), nil
}

// Update the CheCluster ImagePuller spec if the default values are not set
// returns the updated spec and an error during update
func UpdateImagePullerSpecIfEmpty(ctx *DeployContext) (orgv1.CheClusterSpecImagePuller, error) {
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

func CreateKubernetesImagePuller(ctx *DeployContext) (bool, error) {
	imagePuller := GetExpectedKubernetesImagePuller(ctx)
	err := ctx.ClusterAPI.Client.Create(context.TODO(), imagePuller, &client.CreateOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetExpectedKubernetesImagePuller(ctx *DeployContext) *chev1alpha1.KubernetesImagePuller {
	return &chev1alpha1.KubernetesImagePuller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ctx.CheCluster.Name + "-image-puller",
			Namespace: ctx.CheCluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ctx.CheCluster, ctx.CheCluster.GroupVersionKind()),
			},
			Labels: map[string]string{
				"app.kubernetes.io/part-of": ctx.CheCluster.Name,
				"app":                       "che",
				"component":                 "kubernetes-image-puller",
			},
		},
		Spec: ctx.CheCluster.Spec.ImagePuller.Spec,
	}
}

// Unisntall the CSV, OperatorGroup, Subscription, KubernetesImagePuller, and update the CheCluster to remove
// the image puller spec.  Returns true if the CheCluster was updated
func UninstallImagePullerOperator(ctx *DeployContext) (bool, error) {
	// Delete the KubernetesImagePuller
	updated := false
	imagePuller := &chev1alpha1.KubernetesImagePuller{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: ctx.CheCluster.Name + "-image-puller"}, imagePuller)
	if err != nil && !errors.IsNotFound(err) {
		return updated, err
	}
	if imagePuller.Name != "" {
		if err = ctx.ClusterAPI.Client.Delete(context.TODO(), imagePuller, &client.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return updated, err
		}
	}

	// Delete the ClusterServiceVersion
	csv := &operatorsv1alpha1.ClusterServiceVersion{}
	err = ctx.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: DefaultKubernetesImagePullerOperatorCSV()}, csv)
	if err != nil && !errors.IsNotFound(err) {
		return updated, err
	}

	if csv.Name != "" {
		logrus.Infof("Deleting ClusterServiceVersion %v", csv.Name)
		err := ctx.ClusterAPI.NonCachedClient.Delete(context.TODO(), csv, &client.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return updated, err
		}
	}

	// Delete the Subscription
	subscription := &operatorsv1alpha1.Subscription{}
	err = ctx.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, subscription)
	if err != nil && !errors.IsNotFound(err) {
		return updated, err
	}

	if subscription.Name != "" {
		err := ctx.ClusterAPI.NonCachedClient.Delete(context.TODO(), subscription, &client.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return updated, err
		}
	}

	// Delete the OperatorGroup if it was created
	operatorGroup := &operatorsv1.OperatorGroup{}
	err = ctx.ClusterAPI.NonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: ctx.CheCluster.Namespace, Name: "kubernetes-imagepuller-operator"}, operatorGroup)
	if err != nil && !errors.IsNotFound(err) {
		return updated, err
	}

	if operatorGroup.Name != "" {
		err := ctx.ClusterAPI.NonCachedClient.Delete(context.TODO(), operatorGroup, &client.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return updated, err
		}
	}

	// Update CR to remove imagePullerSpec
	if ctx.CheCluster.Spec.ImagePuller.Enable || ctx.CheCluster.Spec.ImagePuller.Spec != (chev1alpha1.KubernetesImagePullerSpec{}) {
		ctx.CheCluster.Spec.ImagePuller.Spec = chev1alpha1.KubernetesImagePullerSpec{}
		err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster, &client.UpdateOptions{})
		if err != nil {
			return updated, err
		}
		updated = true
	}
	return updated, nil
}
