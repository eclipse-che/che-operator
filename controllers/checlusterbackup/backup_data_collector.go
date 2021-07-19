//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package checlusterbackup

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

const (
	BackupMetadataFileName = "backup-info.yaml"
	BackupCheCRFileName    = "che-cr.yaml"
	BackupConfigMapsDir    = "configmaps"
	BackupSecretsDir       = "secrets"
	BackupDatabasesDir     = "db"

	backupFilesPerms = 0600
)

type BackupMetadata struct {
	MetadataFileVersion string `json:"metadataFileVersion"`
	CreationDate        string `json:"creationDate"`
	CheVersion          string `json:"cheVersion"`
	Infrastructure      string `json:"infrastructure"`
	AppsDomain          string `json:"appsDomain"`
	Namespace           string `json:"namespace"`
}

func createBackupMetadataFile(bctx *BackupContext, destDir string) (bool, error) {
	infra := "Kubernetes"
	if util.IsOpenShift {
		infra = "Openshift "
		if util.IsOpenShift4 {
			infra += "4"
		} else {
			infra += "3"
		}
	}

	var appsDomain string
	if util.IsOpenShift {
		host, err := util.GetRouterCanonicalHostname(bctx.r.client, bctx.namespace)
		if err != nil {
			return false, err
		}
		appsDomain = host
	} else {
		// Kubernetes
		appsDomain = bctx.cheCR.Spec.K8s.IngressDomain
	}

	backupMetadata := BackupMetadata{
		MetadataFileVersion: "v1",
		CreationDate:        time.Now().String(),
		CheVersion:          bctx.cheCR.Status.CheVersion,
		Infrastructure:      infra,
		AppsDomain:          appsDomain,
		Namespace:           bctx.namespace,
	}

	data, err := yaml.Marshal(backupMetadata)
	if err != nil {
		return false, err
	}

	backupMetadataFilePath := path.Join(destDir, BackupMetadataFileName)
	if err := ioutil.WriteFile(backupMetadataFilePath, data, backupFilesPerms); err != nil {
		return false, err
	}
	return true, nil
}

// CollectBackupData gathers all Che data that needs to be backuped.
func CollectBackupData(bctx *BackupContext, destDir string) (bool, error) {
	if err := prepareDirectory(destDir); err != nil {
		return true, err
	}

	partsToBackup := []func(*BackupContext, string) (bool, error){
		backupCheCR,
		backupDatabases,
		backupConfigMaps,
		backupSecrets,
		createBackupMetadataFile,
	}

	for _, backupPart := range partsToBackup {
		done, err := backupPart(bctx, destDir)
		if err != nil || !done {
			return done, err
		}
	}

	return true, nil
}

// prepareDirectory makes sure that the directory by given path exists and empty
func prepareDirectory(destDir string) error {
	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		// Destination directory exists
		if err := os.RemoveAll(destDir); err != nil {
			return err
		}
	}
	return os.MkdirAll(destDir, os.ModePerm)
}

func backupCheCR(bctx *BackupContext, destDir string) (bool, error) {
	cheCR := &orgv1.CheCluster{}
	namespacedName := types.NamespacedName{Namespace: bctx.namespace, Name: bctx.cheCR.GetName()}
	if err := bctx.r.client.Get(context.TODO(), namespacedName, cheCR); err != nil {
		return false, err
	}
	util.ClearMetadata(&cheCR.ObjectMeta)

	data, err := yaml.Marshal(cheCR)
	if err != nil {
		return true, err
	}

	crFilePath := path.Join(destDir, BackupCheCRFileName)
	if err := ioutil.WriteFile(crFilePath, data, backupFilesPerms); err != nil {
		return false, err
	}
	return true, nil
}

// Saves Che related postgres databases dumps in db/{dbname}.sql
func backupDatabases(bctx *BackupContext, destDir string) (bool, error) {
	// Prepare separate directory for dumps
	dir := path.Join(destDir, BackupDatabasesDir)
	if err := os.Mkdir(dir, os.ModePerm); err != nil {
		return true, err
	}

	if bctx.cheCR.Spec.Database.ExternalDb {
		// Skip database backup as there is an external server
		return true, nil
	}

	databasesToBackup := []string{
		bctx.cheCR.Spec.Database.ChePostgresDb,
		"keycloak",
	}

	k8sClient := util.GetK8Client()
	postgresPodName, err := k8sClient.GetDeploymentPod(deploy.PostgresName, bctx.namespace)
	if err != nil {
		return false, err
	}

	// Dump all databases in a row to reduce the chance of inconsistent data change
	dumpRemoteCommand := getDumpDatabasesScript(databasesToBackup)
	execReason := "dumping databases: " + strings.Join(databasesToBackup, " ")
	if _, err := k8sClient.DoExecIntoPod(bctx.namespace, postgresPodName, dumpRemoteCommand, execReason); err != nil {
		return false, err
	}

	// Get and seve all dumps from the Postgres container
	for _, dbName := range databasesToBackup {
		execReason := fmt.Sprintf("getting database %s dump", dbName)
		dbDump, err := k8sClient.DoExecIntoPod(bctx.namespace, postgresPodName, getMoveDatabaseDumpScript(dbName), execReason)
		if err != nil {
			return false, err
		}

		dbDumpFilePath := path.Join(dir, dbName+".sql")
		if err := ioutil.WriteFile(dbDumpFilePath, []byte(dbDump), backupFilesPerms); err != nil {
			return false, err
		}
	}

	return true, nil
}

func getDumpDatabasesScript(databases []string) string {
	return "DATABASES='" + strings.Join(databases, " ") + `'
	  DIR=/tmp/che-backup
		rm -rf $DIR && mkdir -p $DIR
		for db in $DATABASES; do
		  pg_dump "$db" > "${DIR}/${db}.sql"
		done
	`
}

// Sends given database dump into stdout and deletes the dump
func getMoveDatabaseDumpScript(dbName string) string {
	return "DBNAME='" + dbName + `'
	  DIR=/tmp/che-backup
	  cat "${DIR}/${DBNAME}.sql"
		rm -f "${DIR}/${DBNAME}.sql" > /dev/null 2>&1
	`
}

func backupConfigMaps(bctx *BackupContext, destDir string) (bool, error) {
	// Prepare separate directory for config maps
	dir := path.Join(destDir, BackupConfigMapsDir)
	if err := os.Mkdir(dir, os.ModePerm); err != nil {
		return true, err
	}

	fakeDeployContext := &deploy.DeployContext{ClusterAPI: deploy.ClusterAPI{Client: bctx.r.client}}
	caBundlesConfigmaps, err := deploy.GetCACertsConfigMaps(fakeDeployContext)
	if err != nil {
		return false, err
	}

	for _, cm := range caBundlesConfigmaps {
		name := cm.GetName()
		util.ClearMetadata(&cm.ObjectMeta)
		data, err := yaml.Marshal(cm)
		if err != nil {
			return false, err
		}
		cmFilePath := path.Join(dir, name+".yaml")
		if err := ioutil.WriteFile(cmFilePath, data, backupFilesPerms); err != nil {
			return false, err
		}
	}

	return true, nil
}

func backupSecrets(bctx *BackupContext, destDir string) (bool, error) {
	// Prepare separate directory for secrets
	dir := path.Join(destDir, BackupSecretsDir)
	if err := os.Mkdir(dir, os.ModePerm); err != nil {
		return true, err
	}

	// Create secrets list to backup
	secretsNames := []string{}
	if bctx.cheCR.Spec.Database.ChePostgresSecret != "" {
		secretsNames = append(secretsNames, bctx.cheCR.Spec.Database.ChePostgresSecret)
	}
	if bctx.cheCR.Spec.Auth.IdentityProviderPostgresSecret != "" {
		secretsNames = append(secretsNames, bctx.cheCR.Spec.Auth.IdentityProviderPostgresSecret)
	}
	if bctx.cheCR.Spec.Auth.IdentityProviderSecret != "" {
		secretsNames = append(secretsNames, bctx.cheCR.Spec.Auth.IdentityProviderSecret)
	}

	// Retrieve and save each secret
	for _, secretName := range secretsNames {
		secret := &corev1.Secret{}
		namespacedName := types.NamespacedName{Name: secretName, Namespace: bctx.namespace}
		if err := bctx.r.client.Get(context.TODO(), namespacedName, secret); err != nil {
			return false, err
		}

		util.ClearMetadata(&secret.ObjectMeta)

		data, err := yaml.Marshal(secret)
		if err != nil {
			return false, err
		}
		secretFilePath := path.Join(dir, secretName+".yaml")
		if err := ioutil.WriteFile(secretFilePath, data, backupFilesPerms); err != nil {
			return false, err
		}
	}

	return true, nil
}
