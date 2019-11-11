package deploy

import (
	"fmt"
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"strconv"
	"testing"
)

type DummyServiceCreator struct {
}

func (s *DummyServiceCreator) CreateService(cr *orgv1.CheCluster, service *corev1.Service) error {
	return nil
}

type DummyFailingServiceCreator struct {
}

func (s *DummyFailingServiceCreator) CreateService(cr *orgv1.CheCluster, service *corev1.Service) error {
	return fmt.Errorf("dummy error")
}

func TestCreateCheDefaultService(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
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

func TestCreateCheServiceEnableMetrics(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheMetrics: true,
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

func TestCreateCheServiceEnableMetricsWithCustomPort(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheMetrics:     true,
				CheMetricsPort: "8887",
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
	checkPort(ports[1], "metrics", 8887, t)
}

func TestDontCreatePortWhenDisableMetricsButSetPort(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheMetrics:     false,
				CheMetricsPort: "8887",
			},
		},
	}

	service, err := NewCheService(cheCluster, map[string]string{}, &DummyServiceCreator{})

	if service == nil || err != nil {
		t.Error("service should be created witn no error")
	}
	ports := service.Spec.Ports
	if len(ports) != 1 {
		t.Error("expected 2 ports")
	}
	checkPort(ports[0], "http", 8080, t)
}

func TestFailWhenCantCreateService(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
			},
		},
	}

	service, err := NewCheService(cheCluster, map[string]string{}, &DummyFailingServiceCreator{})


	if service != nil || err == nil {
		t.Errorf("expected error and service to be nil. Actual service:`%s` err:`%s`", service, err)
	}
}

func TestFailWhenInvalidMetricsPortSet(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheMetrics:     true,
				CheMetricsPort: "this looks like an invalid port",
			},
		},
	}

	service, err := NewCheService(cheCluster, map[string]string{}, &DummyServiceCreator{})

	if service != nil || err == nil {
		t.Errorf("expected error and service to be nil. Actual service:`%s` err:`%s`", service, err)
	}
	if _, errType := err.(*strconv.NumError); !errType {
		t.Errorf("Expected error type `NumError`. Actual type:`%s`", reflect.TypeOf(err))
	}
}

func checkPort(actualPort corev1.ServicePort, expectedName string, expectedPort int32, t *testing.T) {
	if actualPort.Name != expectedName || actualPort.Port != expectedPort {
		t.Errorf("expected port name:`%s` port:`%d`, actual name:`%s` port:`%d`",
			expectedName, expectedPort, actualPort.Name, actualPort.Port)
	}
}
