package server

import "github.com/eclipse/che-operator/pkg/deploy"

func init() {
	err := deploy.InitTestDefaultsFromDeployment("../../../deploy/operator.yaml")
	if err != nil {
		panic(err)
	}
}
