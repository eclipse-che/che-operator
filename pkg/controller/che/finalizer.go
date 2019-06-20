package che

import (
	"context"
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

func (r *ReconcileChe) ReconcileFinalizer(instance *orgv1.CheCluster) (err error) {
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !util.ContainsString(instance.ObjectMeta.Finalizers, oAuthFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, oAuthFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				return err
			}
		}
	} else {
		if util.ContainsString(instance.ObjectMeta.Finalizers, oAuthFinalizerName) {
			oAuthClientName := instance.Spec.Auth.OauthClientName
			logrus.Infof("Custom resource %s is being deleted. Deleting oAuthClient %s first", instance.Name, oAuthClientName)
			oAuthClient, err := r.GetOAuthClient(oAuthClientName)
			if err != nil {
				logrus.Errorf("Failed to get %s oAuthClient: %s", oAuthClientName, err)
				return err
			}
			if err := r.client.Delete(context.TODO(), oAuthClient); err != nil {
				logrus.Errorf("Failed to delete %s oAuthClient: %s", oAuthClientName, err)
				return err
			}
			instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, oAuthFinalizerName)
			logrus.Infof("Updating %s CR", instance.Name)

			if err := r.client.Update(context.Background(), instance); err != nil {
				logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
				return err
			}
		}
		return nil
	}
	return nil
}

func (r *ReconcileChe) DeleteFinalizer(instance *orgv1.CheCluster) (err error) {
	instance.ObjectMeta.Finalizers = util.DoRemoveString(instance.ObjectMeta.Finalizers, oAuthFinalizerName)
	logrus.Infof("Removing OAuth finalizer on %s CR", instance.Name)
	if err := r.client.Update(context.Background(), instance); err != nil {
		logrus.Errorf("Failed to update %s CR: %s", instance.Name, err)
		return err
	}
	return nil
}
