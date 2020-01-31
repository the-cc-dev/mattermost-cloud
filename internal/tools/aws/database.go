package aws

import (
	"fmt"
	"sort"

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

// RDSDatabase is a database backed by AWS RDS.
type RDSDatabase struct {
	installationID string
}

const (
	// RDSErrorSnapshotCreating ...
	RDSErrorSnapshotCreating = "rds database snapshot is being created"
	// RDSErrorSnapshotDeleting ...
	RDSErrorSnapshotDeleting = "rds database snapshot is being deleted"
	// RDSErrorSnapshotModifying ...
	RDSErrorSnapshotModifying = "rds database snapshot is being modified"
)

// NewRDSDatabase returns a new RDSDatabase interface.
func NewRDSDatabase(installationID string) *RDSDatabase {
	return &RDSDatabase{
		installationID: installationID,
	}
}

// Snapshot takes a snapshot of the RDS database.
func (d *RDSDatabase) Snapshot(logger log.FieldLogger) error {
	awsClient := New()
	awsID := CloudID(d.installationID)

	err := awsClient.rdsEnsureDBClusterSnapshotCreated(awsID, []*rds.Tag{&rds.Tag{
		Key:   aws.String(DefaultClusterInstallationSnapshotTagKey),
		Value: aws.String(fmt.Sprintf(DefaultClusterInstallationSnapshotTagValueTemplate, awsID)),
	}})
	if err != nil {
		return err
	}

	return nil
}

// Restore restores database from the most recent snapshot. Optianally, it takes a cluster ID if the
// the intent is to restore the database in another cluster.
func (d *RDSDatabase) Restore(store model.InstallationDatabaseStoreInterface, clusterID string, logger log.FieldLogger) error {
	awsClient := New()
	awsClient.AddSQLStore(store)

	awsID := CloudID(d.installationID)
	lookupTagValue := fmt.Sprintf(DefaultClusterInstallationSnapshotTagValueTemplate, awsID)

	snapshotTagListMap, err := awsClient.rdsGetSnapshotTagsMap([]*rds.Filter{})
	if err != nil {
		return err
	}

	var snapshots []*rds.DBClusterSnapshot
	for snapshot, tags := range *snapshotTagListMap {
		for _, tag := range tags {
			if tag.Key != nil && *tag.Key == DefaultClusterInstallationSnapshotTagKey {
				if tag.Value != nil && *tag.Value != lookupTagValue {
					snapshots = append(snapshots, snapshot)
				}
			}
		}
	}

	if len(snapshots) > 0 {
		return errors.Errorf("DB cluster %s has no snapshots", awsID)
	}

	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].ClusterCreateTime.After(*snapshots[j].ClusterCreateTime)
	})

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
		return errors.Wrapf(err, "unable to get resources required for restoring the RDS database in cluster id %s", clusterID)
	}
	if len(vpcs) != 1 {
		return fmt.Errorf("expected 1 VPC for cluster %s, but got %d", clusterID, len(vpcs))
	}

	switch *snapshots[0].Status {
	case RDSStatusAvailable:
		err = awsClient.rdsEnsureRestoreDBClusterFromSnapshot(*vpcs[0].VpcId, awsID, *snapshots[0].DBClusterSnapshotIdentifier, logger)
		if err != nil {
			return errors.Wrapf(err, "unable to restore RDS database cluster from snapshot %s", *snapshots[0].DBClusterSnapshotIdentifier)
		}
		err = awsClient.rdsEnsureDBClusterInstanceCreated(awsID, fmt.Sprintf("%s-master", awsID), logger)
		if err != nil {
			return errors.Wrapf(err, "unable to restore RDS database instance from snapshot %s", *snapshots[0].DBClusterSnapshotIdentifier)
		}

	case RDSStatusCreating:
		return errors.New(RDSErrorSnapshotCreating)

	case RDSStatusModifying:
		return errors.New(RDSErrorSnapshotModifying)

	case RDSStatusDeleting:
		return errors.New(RDSErrorSnapshotDeleting)

	default:
		return errors.Errorf("unknown snapshot status %s", *snapshots[0].Status)
	}

	return nil
}

// Provision completes all the steps necessary to provision a RDS database.
func (d *RDSDatabase) Provision(store model.InstallationDatabaseStoreInterface, logger log.FieldLogger) error {
	awsClient := New()
	awsClient.AddSQLStore(store)

	awsID := CloudID(d.installationID)

	vpcID, err := getVpcID(d.installationID, awsClient, logger)
	if err != nil {
		return errors.Wrap(err, "unable to get resources required for provisioning an RDS database")
	}

	rdsSecret, err := awsClient.secretsManagerEnsureRDSSecretCreated(awsID, logger)
	if err != nil {
		return err
	}
	err = awsClient.rdsEnsureDBClusterCreated(awsID, vpcID, rdsSecret.MasterUsername, rdsSecret.MasterPassword, logger)
	if err != nil {
		return err
	}

	err = awsClient.rdsEnsureDBClusterInstanceCreated(awsID, fmt.Sprintf("%s-master", awsID), logger)
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
