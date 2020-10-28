package deploy

import "errors"
import operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

func InstallOperatorGroup(deployContext *DeployContext) error {
	return errors.New("hI")
}

func SubscriptionsAreEqual(expected *operatorsv1alpha1.Subscription, actual *operatorsv1alpha1.Subscription) bool {
	return expected.Spec.CatalogSource == actual.Spec.CatalogSource &&
		expected.Spec.CatalogSourceNamespace == actual.Spec.CatalogSourceNamespace &&
		expected.Spec.Channel == actual.Spec.Channel &&
		expected.Spec.InstallPlanApproval == actual.Spec.InstallPlanApproval &&
		expected.Spec.Package == actual.Spec.Package
}
