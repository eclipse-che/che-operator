package che

import (
    "github.com/eclipse-che/che-operator/pkg/deploy"
    "github.com/eclipse-che/che-operator/pkg/deploy/gateway"
    "github.com/eclipse-che/che-operator/pkg/util"
    rbac "k8s.io/api/rbac/v1"
    "k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileChe) reconcileGatewayPermissions(deployContext *deploy.DeployContext) (bool, error) {
    if util.IsNativeUserModeEnabled(deployContext.CheCluster) {
        instance := deployContext.CheCluster

        name := instance.Namespace + "-" + gateway.GatewayServiceName
        if _, err := deploy.SyncClusterRoleToCluster(deployContext, name, getGatewayClusterRoleRules()); err != nil {
            return false, err
        }

        if _, err := deploy.SyncClusterRoleBindingAndAddFinalizerToCluster(deployContext, name, gateway.GatewayServiceName, name); err != nil {
            return false, err
        }
    }

    return true, nil
}

func (r *ReconcileChe) reconcileGatewayPermissionsFinalizers(deployContext *deploy.DeployContext) (bool, error) {
    instance := deployContext.CheCluster
    name := instance.Namespace + "-" + gateway.GatewayServiceName

    if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRole{}); !done {
        return false, err
    }

    if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}); !done {
        return false, err
    }

    return true, nil
}

func getGatewayClusterRoleRules() []rbac.PolicyRule {
    return []rbac.PolicyRule{
        {
            Verbs:     []string{"create"},
            APIGroups: []string{"authentication.k8s.io"},
            Resources: []string{"tokenreviews"},
        },
        {
            Verbs:     []string{"create"},
            APIGroups: []string{"authorization.k8s.io"},
            Resources: []string{"subjectaccessreviews"},
        },
    }
}
