//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package imagepuller

import (
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
)

func init() {
	k8shelper.InitializeForTesting()
	test.EnableTestMode()

	infrastructure.InitializeForTesting(infrastructure.OpenShiftV4)
	defaults.InitializeForTesting("../../../config/manager/manager.yaml")
}
