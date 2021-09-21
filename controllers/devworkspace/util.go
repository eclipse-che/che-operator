package devworkspace

import (
	chev2alpha1 "github.com/eclipse-che/che-operator/api/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

type DevworkspaceState int

const (
	DevworkspaceStateNotPresent DevworkspaceState = 0
	DevworkspaceStateDisabled   DevworkspaceState = 1
	DevworkspaceStateEnabled    DevworkspaceState = 2
	DevworkspaceStateDisabled   = 1
	DevworkspaceStateEnabled    = 2
)

func GetDevworkspaceState(scheme *runtime.Scheme, cr *chev2alpha1.CheCluster) DevworkspaceState {
	if !scheme.IsGroupRegistered("controller.devfile.io") {
		return DevworkspaceStateNotPresent
	}

	if !cr.Spec.IsEnabled() {
		return DevworkspaceStateDisabled
	}

	return DevworkspaceStateEnabled
}
