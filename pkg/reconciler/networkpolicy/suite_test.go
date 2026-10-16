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

//go:build integration

package networkpolicy

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test/envtest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var env *envtest.Env

func TestNetworkPolicy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NetworkPolicy Reconciler Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	env = envtest.Start()
})

var _ = AfterSuite(func() {
	if env != nil {
		env.Stop()
	}
})
