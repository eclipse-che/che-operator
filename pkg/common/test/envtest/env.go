//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package envtest

import (
	"context"
	"os"
	"path/filepath"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crenvtest "sigs.k8s.io/controller-runtime/pkg/envtest"
)

type Env struct {
	testEnv *crenvtest.Environment

	Client          client.Client
	DiscoveryClient discovery.DiscoveryInterface
	Config          *rest.Config
	Scheme          *runtime.Scheme
	Context         context.Context
	Cancel          context.CancelFunc
}

func Start() *Env {
	ginkgo.GinkgoHelper()

	gomega.Expect(os.Getenv("KUBEBUILDER_ASSETS")).NotTo(gomega.BeEmpty(), "set KUBEBUILDER_ASSETS")

	projectRoot, err := utils.FindProjectRoot()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	env := &Env{Scheme: newScheme()}

	env.testEnv = &crenvtest.Environment{
		Scheme:                env.Scheme,
		CRDDirectoryPaths:     []string{filepath.Join(projectRoot, "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	env.Config, err = env.testEnv.Start()
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to start envtest control plane")

	env.Context, env.Cancel = context.WithCancel(context.Background())

	env.Client, err = client.New(env.Config, client.Options{Scheme: env.Scheme})
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create Client")

	env.DiscoveryClient, err = discovery.NewDiscoveryClientForConfig(env.Config)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "failed to create DiscoveryClient")

	return env
}

func (e *Env) Stop() {
	ginkgo.GinkgoHelper()

	e.Cancel()

	gomega.Expect(e.testEnv.Stop()).To(gomega.Succeed(), "failed to stop envtest control plane")
}

func (e *Env) ensureNamespaceExists(name string) error {
	if name == "" {
		return nil
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if err := e.Client.Create(e.Context, namespace); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(chev2.AddToScheme(scheme))

	return scheme
}
