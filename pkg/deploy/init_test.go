package deploy

func init() {
	err := InitTestDefaultsFromDeployment("../../deploy/operator.yaml")
	if err != nil {
		panic(err)
	}
}
