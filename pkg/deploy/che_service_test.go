package deploy

import (
	"fmt"
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

type DummyServiceCreator struct {
}

func (s *DummyServiceCreator) CreateService(cr *orgv1.CheCluster, service *corev1.Service, updateIfExists bool) error {
	return nil
}

type DummyFailingServiceCreator struct {
}

func (s *DummyFailingServiceCreator) CreateService(cr *orgv1.CheCluster, service *corev1.Service, updateIfExists bool) error {
	return fmt.Errorf("dummy error")
}

func TestCreateCheDefaultService(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{},
		},
	}

	service, err := NewCheService(cheCluster, map[string]string{}, &DummyServiceCreator{})

	if service == nil || err != nil {
		t.Error("service should be created witn no error")
	}
	ports := service.Spec.Ports
	if len(ports) != 1 {
		t.Error("expected 1 default port")
	}
	checkPort(ports[0], "http", 8080, t)
}

func TestCreateCheServiceEnableMetrics(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Metrics: orgv1.CheClusterSpecMetrics{
				Enable: false,
			},
		},
	}

	service, err := NewCheService(cheCluster, map[string]string{}, &DummyServiceCreator{})

	if service == nil || err != nil {
		t.Error("service should be created witn no error")
	}
	ports := service.Spec.Ports
	if len(ports) != 1 {
		t.Error("expected 1 default port")
	}
	checkPort(ports[0], "http", 8080, t)
}

func TestCreateCheServiceDisableMetrics(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Metrics: orgv1.CheClusterSpecMetrics{
				Enable: true,
			},
		},
	}

	service, err := NewCheService(cheCluster, map[string]string{}, &DummyServiceCreator{})

	if service == nil || err != nil {
		t.Error("service should be created witn no error")
	}
	ports := service.Spec.Ports
	if len(ports) != 2 {
		t.Error("expected 2 ports")
	}
	checkPort(ports[0], "http", 8080, t)
	checkPort(ports[1], "metrics", 8087, t)
}

func TestFailWhenCantCreateService(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{},
		},
	}

	service, err := NewCheService(cheCluster, map[string]string{}, &DummyFailingServiceCreator{})

	if service != nil || err == nil {
		t.Errorf("expected error and service to be nil. Actual service:`%s` err:`%s`", service, err)
	}
}


func checkPort(actualPort corev1.ServicePort, expectedName string, expectedPort int32, t *testing.T) {
	if actualPort.Name != expectedName || actualPort.Port != expectedPort {
		t.Errorf("expected port name:`%s` port:`%d`, actual name:`%s` port:`%d`",
			expectedName, expectedPort, actualPort.Name, actualPort.Port)
	}
}
