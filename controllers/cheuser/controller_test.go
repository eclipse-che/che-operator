package cheuser

import (
	"context"
	"sync"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	v1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/controllers/devworkspace"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	projectv1 "github.com/openshift/api/project/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func setupCheCluster(t *testing.T, ctx context.Context, cl client.Client, scheme *runtime.Scheme, cheNamespaceName string, cheName string) {
	var cheNamespace metav1.Object
	if infrastructure.IsOpenShift() {
		cheNamespace = &projectv1.Project{}
	} else {
		cheNamespace = &corev1.Namespace{}
	}

	cheNamespace.SetName(cheNamespaceName)
	if err := cl.Create(ctx, cheNamespace.(runtime.Object)); err != nil {
		t.Fatal(err)
	}

	cheCluster := v1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheName,
			Namespace: cheNamespaceName,
		},
		Spec: v1.CheClusterSpec{
			Server: v1.CheClusterSpecServer{
				CheHost: "che-host",
				CustomCheProperties: map[string]string{
					"CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX": "root-domain",
				},
			},
			DevWorkspace: v1.CheClusterSpecDevWorkspace{
				Enable: true,
			},
			K8s: v1.CheClusterSpecK8SOnly{
				IngressDomain: "root-domain",
			},
		},
	}
	if err := cl.Create(ctx, &cheCluster); err != nil {
		t.Fatal(err)
	}

	// also create the self-signed-certificate secret to pretend we have TLS set up
	cert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.CheTLSSelfSignedCertificateSecretName,
			Namespace: cheNamespaceName,
		},
		Data: map[string][]byte{
			"ca.crt":     []byte("my certificate"),
			"other.data": []byte("should not be copied to target ns"),
		},
	}
	if err := cl.Create(ctx, cert); err != nil {
		t.Fatal(err)
	}

	r := devworkspace.New(cl, scheme)
	// the reconciliation needs to run twice for it to be trully finished - we're setting up finalizers etc...
	if _, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: cheName, Namespace: cheNamespaceName}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: cheName, Namespace: cheNamespaceName}}); err != nil {
		t.Fatal(err)
	}
}

func setup(infraType infrastructure.Type, objs ...runtime.Object) (*runtime.Scheme, client.Client, *CheUserNamespaceReconciler) {
	infrastructure.InitializeForTesting(infraType)
	devworkspace.CleanCheClusterInstancesForTest()
	util.IsOpenShift = infraType == infrastructure.OpenShiftv4
	util.IsOpenShift4 = infraType == infrastructure.OpenShiftv4

	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, objs...)

	r := &CheUserNamespaceReconciler{
		client: cl,
		scheme: scheme,
		namespaceCache: namespaceCache{
			client:          cl,
			knownNamespaces: map[string]namespaceInfo{},
			lock:            sync.Mutex{},
		},
	}

	return scheme, cl, r
}

func TestSkipsUnlabeledNamespaces(t *testing.T) {
	test := func(t *testing.T, infraType infrastructure.Type, namespace metav1.Object) {
		ctx := context.TODO()
		scheme, cl, r := setup(infraType, namespace.(runtime.Object))
		setupCheCluster(t, ctx, cl, scheme, "che", "che")

		if _, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}}); err != nil {
			t.Fatal(err)
		}

		// no new secret or configmap should be created in the namespace
		ss := &corev1.SecretList{}
		if err := cl.List(ctx, ss, client.InNamespace(namespace.GetName())); err != nil {
			t.Fatal(err)
		}
		if len(ss.Items) > 0 {
			t.Errorf("No secrets expected in the tested namespace but found %d", len(ss.Items))
		}

		cs := &corev1.ConfigMapList{}
		if err := cl.List(ctx, cs, client.InNamespace(namespace.GetName())); err != nil {
			t.Fatal(err)
		}
		if len(cs.Items) > 0 {
			t.Errorf("No configmaps expected in the tested namespace but found %d", len(cs.Items))
		}
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, infrastructure.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, infrastructure.OpenShiftv4, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prj",
			},
		})
	})
}

func TestRequiresLabelsToMatchOneOfMultipleCheCluster(t *testing.T) {
	test := func(t *testing.T, infraType infrastructure.Type, namespace metav1.Object) {
		ctx := context.TODO()
		scheme, cl, r := setup(infraType, namespace.(runtime.Object))
		setupCheCluster(t, ctx, cl, scheme, "che1", "che")
		setupCheCluster(t, ctx, cl, scheme, "che2", "che")

		res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}})
		if err != nil {
			t.Fatal(err)
		}

		if !res.Requeue {
			t.Error("The reconciliation request should have been requeued.")
		}
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, infrastructure.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
				},
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, infrastructure.OpenShiftv4, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prj",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
				},
			},
		})
	})
}

func TestMatchingCheClusterCanBeSelectedUsingLabels(t *testing.T) {
	test := func(t *testing.T, infraType infrastructure.Type, namespace metav1.Object) {
		ctx := context.TODO()
		scheme, cl, r := setup(infraType, namespace.(runtime.Object))
		setupCheCluster(t, ctx, cl, scheme, "che1", "che")
		setupCheCluster(t, ctx, cl, scheme, "che2", "che")

		res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}})
		if err != nil {
			t.Fatal(err)
		}

		if res.Requeue {
			t.Error("The reconciliation request should have succeeded but is requesting a requeue.")
		}
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, infrastructure.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
					cheNameLabel:                    "che",
					cheNamespaceLabel:               "che1",
				},
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, infrastructure.OpenShiftv4, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prj",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
					cheNameLabel:                    "che",
					cheNamespaceLabel:               "che1",
				},
			},
		})
	})
}

func TestCreatesDataInNamespace(t *testing.T) {
	test := func(t *testing.T, infraType infrastructure.Type, namespace metav1.Object, objs ...runtime.Object) {
		ctx := context.TODO()
		allObjs := append(objs, namespace.(runtime.Object))
		scheme, cl, r := setup(infraType, allObjs...)
		setupCheCluster(t, ctx, cl, scheme, "eclipse-che", "che")

		res, err := r.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: namespace.GetName()}})
		if err != nil {
			t.Fatal(err)
		}

		if res.Requeue {
			t.Error("The reconciliation request should have succeeded.")
		}

		proxySettings := corev1.ConfigMap{}
		if err := cl.Get(ctx, client.ObjectKey{Name: "che-eclipse-che-proxy-settings", Namespace: namespace.GetName()}, &proxySettings); err != nil {
			t.Fatal(err)
		}
		if proxySettings.GetAnnotations()[constants.DevWorkspaceMountAsAnnotation] != "env" {
			t.Errorf("proxy settings should be annotated as mount as 'env' but was '%s'", proxySettings.GetLabels()[constants.DevWorkspaceMountAsAnnotation])
		}
		if proxySettings.GetLabels()[constants.DevWorkspaceMountLabel] != "true" {
			t.Errorf("proxy settings should be labeled as mounted was '%s'", proxySettings.GetLabels()[constants.DevWorkspaceMountAsAnnotation])
		}
		if len(proxySettings.Data) != 1 {
			t.Errorf("Expecting just 1 element in the default proxy settings")
		}
		if proxySettings.Data["NO_PROXY"] != ".svc" {
			t.Errorf("Unexpected proxy settings")
		}

		cert := corev1.Secret{}
		if err := cl.Get(ctx, client.ObjectKey{Name: "che-eclipse-che-cert", Namespace: namespace.GetName()}, &cert); err != nil {
			t.Fatal(err)
		}
		if cert.GetAnnotations()[constants.DevWorkspaceMountAsAnnotation] != "file" {
			t.Errorf("proxy settings should be annotated as mount as 'env' but was '%s'", proxySettings.GetLabels()[constants.DevWorkspaceMountAsAnnotation])
		}
		if cert.GetAnnotations()[constants.DevWorkspaceMountPathAnnotation] != "/tmp/che/secret/" {
			t.Errorf("proxy settings should be annotated as mount as 'env' but was '%s'", proxySettings.GetLabels()[constants.DevWorkspaceMountAsAnnotation])
		}
		if cert.GetLabels()[constants.DevWorkspaceMountLabel] != "true" {
			t.Errorf("proxy settings should be labeled as mounted was '%s'", proxySettings.GetLabels()[constants.DevWorkspaceMountAsAnnotation])
		}
		if len(cert.Data) != 1 {
			t.Errorf("Expecting just 1 element in the self-signed cert")
		}
		if string(cert.Data["ca.crt"]) != "my certificate" {
			t.Errorf("Unexpected self-signed certificate")
		}
	}

	t.Run("k8s", func(t *testing.T) {
		test(t, infrastructure.Kubernetes, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ns",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
				},
			},
		})
	})

	t.Run("openshift", func(t *testing.T) {
		test(t, infrastructure.OpenShiftv4, &projectv1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prj",
				Labels: map[string]string{
					workspaceNamespaceOwnerUidLabel: "uid",
				},
			},
		}, &configv1.Proxy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: configv1.ProxySpec{
				NoProxy: ".svc",
			},
			Status: configv1.ProxyStatus{
				NoProxy: ".svc",
			},
		})
	})
}
