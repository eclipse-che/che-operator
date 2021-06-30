package che

import (
	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/util"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileChe) reconcileGatewayPermissions(deployContext *deploy.DeployContext) (bool, error) {
	if util.IsNativeUserModeEnabled(deployContext.CheCluster) {
		name := gatewayPermisisonsName(deployContext.CheCluster)
		if _, err := deploy.SyncClusterRoleToCluster(deployContext, name, getGatewayClusterRoleRules()); err != nil {
			return false, err
		}

		if _, err := deploy.SyncClusterRoleBindingAndAddFinalizerToCluster(deployContext, name, gateway.GatewayServiceName, name); err != nil {
			return false, err
		}
	} else {
		return deleteGatewayPermissions(deployContext)
	}

	return true, nil
}

func (r *ReconcileChe) reconcileGatewayPermissionsFinalizers(deployContext *deploy.DeployContext) (bool, error) {
	if !deployContext.CheCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return deleteGatewayPermissions(deployContext)
	}

	return true, nil
}

func deleteGatewayPermissions(deployContext *deploy.DeployContext) (bool, error) {
	name := gatewayPermisisonsName(deployContext.CheCluster)
	if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRole{}); !done {
		return false, err
	}

	if done, err := deploy.Delete(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}); !done {
		return false, err
	}

	return true, nil
}

func gatewayPermisisonsName(instance *orgv1.CheCluster) string {
	return instance.Namespace + "-" + gateway.GatewayServiceName
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
