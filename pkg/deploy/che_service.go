package deploy

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"strconv"
)

type ServiceCreator interface {
	CreateService(cr *orgv1.CheCluster, service *corev1.Service) error
}

func NewCheService(instance *orgv1.CheCluster, cheLabels map[string]string, r ServiceCreator) (*corev1.Service, error) {
	portNames := []string{"http"}
	portPorts := []int32{8080}
	if instance.Spec.Metrics.Enable {
		portNames = append(portNames, "metrics")

		metricsPort, err := strconv.ParseInt(DefaultCheMetricsPort, 10, 32)
		if err != nil {
			return nil, err
		}

		portPorts = append(portPorts, int32(metricsPort))
	}
	cheService := NewService(instance, "che-host", portNames, portPorts, cheLabels)
	if err := r.CreateService(instance, cheService); err != nil {
		return nil, err
	}
	return cheService, nil
}
