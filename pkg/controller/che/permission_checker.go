//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:	// Contributors:
//   Red Hat, Inc. - initial API and implementation	//   Red Hat, Inc. - initial API and implementation
//

package che

import (
	"fmt"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	authorizationv1 "k8s.io/api/authorization/v1"
	rbac "k8s.io/api/rbac/v1"
)

func getNotPermittedPolicyRules(policies []rbac.PolicyRule, namespace string) ([]rbac.PolicyRule, error) {
	var notPermittedPolicyRules []rbac.PolicyRule = []rbac.PolicyRule{}
	for _, policy := range policies {
		for _, apiGroup := range policy.APIGroups {
			for _, verb := range policy.Verbs {
				for _, resource := range policy.Resources {
					logrus.Infof("Go %s %s %s", apiGroup, verb, resource)
					resourceAttribute := &authorizationv1.ResourceAttributes{
						Namespace: namespace,
						Verb:      verb,
						Group:     apiGroup,
						Resource:  resource,
					}
					logrus.Infof("permission %v", resourceAttribute)
					ok, err := util.K8sclient.IsResourceOperationPermitted(resourceAttribute)
					if err != nil {
						return notPermittedPolicyRules, fmt.Errorf("failed to check policy rule: %v", policy)
					}
					if !ok {
						if len(notPermittedPolicyRules) == 0 {
							notPermittedPolicyRules = append(notPermittedPolicyRules, policy)
						} else {
							lastNotPermittedRule := notPermittedPolicyRules[len(notPermittedPolicyRules)-1]
							if lastNotPermittedRule.Resources[0] != policy.Resources[0] && lastNotPermittedRule.APIGroups[0] != policy.APIGroups[0] {
								notPermittedPolicyRules = append(notPermittedPolicyRules, policy)
							}
						}
					}
				}
			}
		}
	}
	return notPermittedPolicyRules, nil
}
