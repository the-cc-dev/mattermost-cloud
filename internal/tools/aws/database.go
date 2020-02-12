package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/mattermost/mattermost-cloud/model"
	mmv1alpha1 "github.com/mattermost/mattermost-operator/pkg/apis/mattermost/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const connStringTemplate = "mysql://%s:%s@tcp(%s:3306)/mattermost?charset=utf8mb4,utf8&readTimeout=30s&writeTimeout=30s"

// RDSDatabase is a database backed by AWS RDS.
type RDSDatabase struct {
	installationID string
	dbClusterID    string
	dbInstanceID   string
	dbSecretName   string
	awsClient      *Client
}

// NewRDSDatabase returns a new RDSDatabase interface.
func NewRDSDatabase(installationID string, awsClient *Client) *RDSDatabase {
	return &RDSDatabase{
		installationID: installationID,
		dbClusterID:    CloudID(installationID),
		dbInstanceID:   fmt.Sprintf("%s-master", CloudID(installationID)),
		dbSecretName:   fmt.Sprintf("%s-rds", CloudID(installationID)),
		awsClient:      awsClient,
	}
}

// Provision completes all the steps necessary to provision a RDS database.
func (d *RDSDatabase) Provision(store model.InstallationDatabaseStoreInterface, logger log.FieldLogger) error {

	clusterInstallations, err := store.GetClusterInstallations(&model.ClusterInstallationFilter{
		PerPage:        model.AllPerPage,
		InstallationID: d.installationID,
	})
	if err != nil {
		return errors.Wrapf(err, "unable to lookup cluster installations for installation %s", d.installationID)
	}

	clusterInstallationCount := len(clusterInstallations)
	if clusterInstallationCount == 0 {
		return fmt.Errorf("no cluster installations found for %s", d.installationID)
	}
	if clusterInstallationCount != 1 {
		return fmt.Errorf("RDS provisioning is not currently supported for multiple cluster installations (found %d)", clusterInstallationCount)
	}

	clusterID := clusterInstallations[0].ClusterID
	vpcs, err := GetVpcsWithFilters([]*ec2.Filter{
		{
			Name:   aws.String(VpcClusterIDTagKey),
			Values: []*string{aws.String(clusterID)},
		},
		{
			Name:   aws.String(VpcAvailableTagKey),
			Values: []*string{aws.String(VpcAvailableTagValueFalse)},
		},
	})
	if err != nil {
		return errors.Wrapf(err, "unable to find a VPC for installation %s", d.installationID)
	}
	if len(vpcs) != 1 {
		return fmt.Errorf("expected 1 VPC for cluster %s, but got %d", clusterID, len(vpcs))
	}
	vpcID := *vpcs[0].VpcId

	rdsSecret, err := d.awsClient.secretsManagerEnsureRDSSecretCreated(d.dbClusterID, logger)
	if err != nil {
		return err
	}
	err = d.awsClient.rdsEnsureDBClusterCreated(d.dbClusterID, vpcID, rdsSecret.MasterUsername, rdsSecret.MasterPassword, logger)
	if err != nil {
		return err
	}

	err = d.awsClient.rdsEnsureDBClusterInstanceCreated(d.dbClusterID, d.dbInstanceID, logger)
	if err != nil {
		return err
	}

	return nil
}

// Snapshot takes a snapshot of the RDS database.
func (d *RDSDatabase) Snapshot(logger log.FieldLogger) error {
	_, err := d.awsClient.RDS.CreateDBClusterSnapshot(&rds.CreateDBClusterSnapshotInput{
		DBClusterIdentifier:         aws.String(d.dbClusterID),
		DBClusterSnapshotIdentifier: aws.String(fmt.Sprintf("%s-snapshot-%s", d.dbClusterID, model.NewID())),
		Tags: []*rds.Tag{&rds.Tag{
			Key:   aws.String(DefaultClusterInstallationSnapshotTagKey),
			Value: aws.String(fmt.Sprintf(DefaultClusterInstallationSnapshotTagValueTemplate, CloudID(d.installationID))),
		}},
	})
	if err != nil {
		return errors.Wrap(err, "failed to create a DB cluster snapshot for replication")
	}

	logger.WithField("db-cluster-name", d.dbClusterID).Info("RDS database snapshot in progress")

	return nil
}

// Teardown removes all AWS resources related to a RDS database.
func (d *RDSDatabase) Teardown(keepData bool, logger log.FieldLogger) error {
	logger.Info("Tearing down AWS RDS database")

	err := d.awsClient.secretsManagerEnsureRDSSecretDeleted(d.dbClusterID, logger)
	if err != nil {
		return errors.Wrap(err, "unable to delete RDS secret")
	}

	if !keepData {
		err = d.awsClient.rdsEnsureDBClusterDeleted(d.dbClusterID, logger)
		if err != nil {
			return errors.Wrap(err, "unable to delete RDS DB cluster")
		}
		logger.WithField("db-cluster-name", d.dbClusterID).Debug("AWS RDS DB cluster deleted")
	} else {
		logger.WithField("db-cluster-name", d.dbClusterID).Info("AWS RDS DB cluster was left intact due to the keep-data setting of this server")
	}

	return nil
}

// GenerateDatabaseSpecAndSecret creates the k8s database spec and secret for
// accessing the RDS database.
func (d *RDSDatabase) GenerateDatabaseSpecAndSecret(logger log.FieldLogger) (*mmv1alpha1.Database, *corev1.Secret, error) {
	rdsSecret, err := secretsManagerGetRDSSecret(d.dbClusterID)
	if err != nil {
		return nil, nil, err
	}

	dbCluster, err := rdsGetDBCluster(d.dbClusterID, logger)
	if err != nil {
		return nil, nil, err
	}

	databaseSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: d.dbSecretName,
		},
		StringData: map[string]string{
			"DB_CONNECTION_STRING": fmt.Sprintf(connStringTemplate, rdsSecret.MasterUsername, rdsSecret.MasterPassword, *dbCluster.Endpoint),
		},
	}

	databaseSpec := &mmv1alpha1.Database{
		Secret: d.dbSecretName,
	}

	logger.Debug("Cluster installation configured to use an AWS RDS Database")

	return databaseSpec, databaseSecret, nil
}
