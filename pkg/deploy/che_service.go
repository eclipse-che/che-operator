package deploy

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
)

type ServiceWriter interface {
	CreateService(cr *orgv1.CheCluster, service *corev1.Service, updateIfExists bool) error
}

func NewCheService(instance *orgv1.CheCluster, cheLabels map[string]string, r ServiceWriter) (*corev1.Service, error) {
	portNames := []string{"http"}
	portPorts := []int32{8080}
	if instance.Spec.Metrics.Enable {
		portNames = append(portNames, "metrics")
		portPorts = append(portPorts, DefaultCheMetricsPort)
	}
	cheService := NewService(instance, "che-host", portNames, portPorts, cheLabels)
	if err := r.CreateService(instance, cheService, true); err != nil {
		return nil, err
	}
	return cheService, nil
}
