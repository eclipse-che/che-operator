package deploy

import ()

func InitDefaultsFromDeployment(deploymentFile string) {
	operator := &appsv1.Deployment{}
	data, err := ioutil.ReadFile(deploymentFile)
	yaml.Unmarshal(data, operator)
	if err == nil {
		for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
			os.Setenv(env.Name, env.Value)
		}
	}

	InitDefaultsFromEnv()
}
