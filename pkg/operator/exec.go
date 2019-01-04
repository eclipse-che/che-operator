package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
)

var (
	pgCommand = "psql -c \"CREATE USER keycloak WITH PASSWORD '" + keycloakPostgresPassword + "'\" " +
		"&& psql -c \"CREATE DATABASE keycloak\" " +
		"&& psql -c \"GRANT ALL PRIVILEGES ON DATABASE keycloak TO keycloak\" " +
		"&& psql -c \"ALTER USER " + chePostgresUser + " WITH SUPERUSER\""
)

func ExecIntoPod(podName string, provisionCommand string) {
	k8s := GetK8SConfig()
	command := []string{"/bin/bash", "-c", provisionCommand}
	logrus.Infof("Provisioning resources in pod %s", podName)
	// print std if operator is run in debug mode (TODO)
	_, stderr, err := k8s.ExecToPod(command, podName, util.GetNamespace())
	if err != nil {
		logrus.Errorf("Error exec'ing into pod %v: , command: %s", err, command)
		logrus.Fatalf(stderr)
	}
	logrus.Info("Provisioning completed")
}
