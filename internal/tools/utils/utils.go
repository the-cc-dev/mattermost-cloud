package utils

import (
	"fmt"
	"io"
	"os"

	"github.com/mattermost/mattermost-cloud/internal/tools/aws"
	"github.com/mattermost/mattermost-cloud/model"
)

// CopyDirectory copy the entire directory to another destination
func CopyDirectory(source string, dest string) error {
	sourceinfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dest, sourceinfo.Mode())
	if err != nil {
		return err
	}

	directory, _ := os.Open(source)

	objects, err := directory.Readdir(-1)

	for _, obj := range objects {

		sourcefilepointer := source + "/" + obj.Name()

		destinationfilepointer := dest + "/" + obj.Name()

		if obj.IsDir() {
			err = CopyDirectory(sourcefilepointer, destinationfilepointer)
			if err != nil {
				fmt.Println(err)
			}
		} else {
			err = copyFile(sourcefilepointer, destinationfilepointer)
			if err != nil {
				fmt.Println(err)
			}
		}

	}
	return nil
}

func copyFile(source string, dest string) error {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}

	defer sourcefile.Close()

	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}

	defer destfile.Close()

	_, err = io.Copy(destfile, sourcefile)
	if err == nil {
		sourceinfo, err := os.Stat(source)
		if err != nil {
			err = os.Chmod(dest, sourceinfo.Mode())
		}

	}

	return nil
}

// GetFilestore returns the Filestore interface that matches the installation.
func GetFilestore(i *model.Installation, awsClient *aws.Client) model.Filestore {
	switch i.Filestore {
	case model.InstallationFilestoreMinioOperator:
		return model.NewMinioOperatorFilestore()
	case model.InstallationFilestoreAwsS3:
		return aws.NewS3Filestore(i.ID, awsClient)
	}

	return &model.UnsupportedFilestore{}
}

// GetDatabase returns the Database interface that matches the installation.
func GetDatabase(installation *model.Installation, awsClient *aws.Client) model.Database {
	switch installation.Database {
	case model.InstallationDatabaseMysqlOperator:
		return model.NewMysqlOperatorDatabase()
	case model.InstallationDatabaseAwsRDS:
		return aws.NewRDSDatabase(installation.ID, awsClient)
	}

	return &model.NotSupportedDatabase{}
}

// GetDatabaseMigration returns the Database interface that matches the cluster installation migration.
func GetDatabaseMigration(installation *model.Installation, clusterInstallation *model.ClusterInstallation, awsClient *aws.Client) model.DatabaseMigration {
	switch installation.Database {
	case model.InstallationDatabaseAwsRDS:
		return aws.NewRDSDatabaseMigration(installation.ID, clusterInstallation.ClusterID, awsClient)
	}

	return &model.NotSupportedDatabaseMigration{}
}
