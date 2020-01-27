package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/mattermost/mattermost-cloud/model"
	mmv1alpha1 "github.com/mattermost/mattermost-operator/pkg/apis/mattermost/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RDSDatabase is a database backed by AWS RDS.
type RDSDatabase struct {
	installationID       string
	parentInstallationID string
}

// NewRDSDatabaseMigration returns a new RDSDatabase interface.
func NewRDSDatabaseMigration(installationID, parentInstallationID string) *RDSDatabase {
	return &RDSDatabase{
		installationID:       installationID,
		parentInstallationID: parentInstallationID,
	}
}

// NewRDSDatabase returns a new RDSDatabase interface.
func NewRDSDatabase(installationID string) *RDSDatabase {
	return &RDSDatabase{
		installationID: installationID,
	}
}

// Provision completes all the steps necessary to provision a RDS database.
func (d *RDSDatabase) Provision(store model.InstallationDatabaseStoreInterface, logger log.FieldLogger) error {
	awsClient := New()
	awsClient.AddSQLStore(store)

	awsID := CloudID(d.installationID)
	logger.Infof("Provisioning AWS RDS cluster with ID %s", awsID)

	vpcID, err := getVpcID(d.installationID, awsClient, logger)
	if err != nil {
		return errors.Wrap(err, "unable to get resources required for provisioning an RDS database")
	}

	masterInstanceName := fmt.Sprintf("%s-master", awsID)

	if d.parentInstallationID == "" {
		rdsSecret, err := awsClient.secretsManagerEnsureRDSSecretCreated(awsID, logger)
		if err != nil {
			return err
		}

		err = awsClient.rdsEnsureDBClusterCreated(awsID, vpcID, rdsSecret.MasterUsername, rdsSecret.MasterPassword, logger)
		if err != nil {
			return err
		}
	} else {
		// TODO(gsagula): If we want to wait for the snapshot here, modify Provision interface to take a context.
		ctx := context.Background()
		awsParentID := CloudID(d.parentInstallationID)

		logger.Infof("snapshotting RDS Database: %s. This may take several minutes depending on the size of the database.", awsParentID)
		out, err := awsClient.createDBClusterSnapshot(ctx, awsParentID, logger)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
		defer cancel()

		status := awsClient.ensureDBClusterSnapshotStatus(ctx, *out.DBClusterSnapshotIdentifier, RDSStatusAvailable, 30*time.Second)
		defer status.close()
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case <-status.Ok():
				err = awsClient.rdsEnsureDBClusterCreatedFromSnapshot(vpcID, awsID, *out.DBClusterSnapshotIdentifier, logger)
				if err != nil {
					return err
				}
				break

			case err := <-status.Error():
				return err

			default:
				logger.Debugf("provisioner is still snapshotting the RDS Database: %s", awsParentID)
			}
		}
	}

	err = awsClient.rdsEnsureDBClusterInstanceCreated(awsID, masterInstanceName, logger)
	if err != nil {
		return err
	}

	return nil
}

// Teardown removes all AWS resources related to a RDS database.
func (d *RDSDatabase) Teardown(keepData bool, logger log.FieldLogger) error {
	err := rdsDatabaseTeardown(d.installationID, keepData, logger)
	if err != nil {
		return errors.Wrap(err, "unable to teardown RDS database")
	}

	return nil
}

// GenerateDatabaseSpecAndSecret creates the k8s database spec and secret for
// accessing the RDS database.
func (d *RDSDatabase) GenerateDatabaseSpecAndSecret(logger log.FieldLogger) (*mmv1alpha1.Database, *corev1.Secret, error) {
	awsID := CloudID(d.installationID)

	rdsSecret, err := secretsManagerGetRDSSecret(awsID)
	if err != nil {
		return nil, nil, err
	}

	dbCluster, err := rdsGetDBCluster(awsID, logger)
	if err != nil {
		return nil, nil, err
	}

	databaseSecretName := fmt.Sprintf("%s-rds", d.installationID)
	databaseConnectionString := fmt.Sprintf(
		"mysql://%s:%s@tcp(%s:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s&writeTimeout=30s",
		rdsSecret.MasterUsername, rdsSecret.MasterPassword, *dbCluster.Endpoint,
	)

	databaseSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: databaseSecretName,
		},
		StringData: map[string]string{
			"DB_CONNECTION_STRING": databaseConnectionString,
		},
	}

	databaseSpec := &mmv1alpha1.Database{
		Secret: databaseSecretName,
	}

	logger.Debug("Cluster installation configured to use an AWS RDS Database")

	return databaseSpec, databaseSecret, nil
}

func rdsDatabaseTeardown(installationID string, keepData bool, logger log.FieldLogger) error {
	logger.Info("Tearing down AWS RDS database")

	a := New()
	awsID := CloudID(installationID)

	err := a.secretsManagerEnsureRDSSecretDeleted(awsID, logger)
	if err != nil {
		return errors.Wrap(err, "unable to delete RDS secret")
	}

	if !keepData {
		err = a.rdsEnsureDBClusterDeleted(awsID, logger)
		if err != nil {
			return errors.Wrap(err, "unable to delete RDS DB cluster")
		}
		logger.WithField("db-cluster-name", awsID).Debug("AWS RDS DB cluster deleted")
	} else {
		logger.WithField("db-cluster-name", awsID).Info("AWS RDS DB cluster was left intact due to the keep-data setting of this server")
	}

	return nil
}

func getVpcID(installationID string, awsClient *Client, logger log.FieldLogger) (string, error) {
	// To properly provision the database we need a SQL client to lookup which
	// cluster(s) the installation is running on.
	if !awsClient.HasSQLStore() {
		return "", errors.New("the provided AWS client does not have SQL store access")
	}

	clusterInstallations, err := awsClient.store.GetClusterInstallations(&model.ClusterInstallationFilter{
		PerPage:        model.AllPerPage,
		InstallationID: installationID,
	})
	if err != nil {
		return "", errors.Wrapf(err, "unable to lookup cluster installations for installation %s", installationID)
	}

	clusterInstallationCount := len(clusterInstallations)
	if clusterInstallationCount == 0 {
		return "", fmt.Errorf("no cluster installations found for %s", installationID)
	}
	if clusterInstallationCount != 1 {
		return "", fmt.Errorf("RDS provisioning is not currently supported for multiple cluster installations (found %d)", clusterInstallationCount)
	}

	clusterID := clusterInstallations[0].ClusterID
	vpcFilters := []*ec2.Filter{
		{
			Name:   aws.String(VpcClusterIDTagKey),
			Values: []*string{aws.String(clusterID)},
		},
		{
			Name:   aws.String(VpcAvailableTagKey),
			Values: []*string{aws.String(VpcAvailableTagValueFalse)},
		},
	}
	vpcs, err := GetVpcsWithFilters(vpcFilters)
	if err != nil {
		return "", err
	}
	if len(vpcs) != 1 {
		return "", fmt.Errorf("expected 1 VPC for cluster %s, but got %d", clusterID, len(vpcs))
	}

	return *vpcs[0].VpcId, nil
}
