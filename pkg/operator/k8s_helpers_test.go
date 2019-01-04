package operator

import (
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

var (
	fakeK8s = fakeClientSet()
)

func fakeClientSet() *k8s {
	client := k8s{}
	client.clientset = fake.NewSimpleClientset()
	return &client
}

func TestGetDeploymentPod(t *testing.T) {

	// create a fake pod
	_, err := fakeK8s.clientset.CoreV1().Pods("eclipse-che").Create(&corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-pod",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app": "postgres",
			},
		},
	})
	if err != nil {
		panic(err)
	}
	pod := fakeK8s.GetDeploymentPod("postgres")
	if len(pod) == 0 {
		logrus.Fatal("Test failed. No pods found by label")
	}
	logrus.Infof("Test passed. Pod %s found", pod)
}

func TestGetEvents(t *testing.T) {

	// fire up an event with fake-pod as involvedObject
	message := "This is a fake event about a fake pod"
	_, err := fakeK8s.clientset.CoreV1().Events("eclipse-che").Create(&corev1.Event{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Event",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-event",
			Namespace: "eclipse-che",
		},
		InvolvedObject: corev1.ObjectReference{
			FieldPath: "spec.containers{fake-pod}",
			Kind:      "Pod",
		},
		Message: message,
		Reason:  "Testing event filtering",
		Type:    "Normal",
	})

	if err != nil {
		panic(err)
	}

	events := fakeK8s.GetEvents("fake-pod")
	// test if function returns any events
	fakePodEvents := events.Items
	if len(fakePodEvents) == 0 {
		logrus.Fatal("Test failed No events found")
	} else {
		logrus.Infof("Test passed. Found %v event", len(fakePodEvents))
	}
	// test if event message matches
	fakePodEventMessage := events.Items[0].Message
	if len(fakePodEventMessage) != len(message) {
		logrus.Fatalf("Test failed. Message to be received: %s, but got %s ", message, fakePodEventMessage)
	} else {
		logrus.Infof("Test passed. Expected event message: %s. Received event message %s", message, fakePodEventMessage)
	}
}
