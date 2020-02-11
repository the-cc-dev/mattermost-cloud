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
)

// RDSDatabaseMigration is a migrated database backed by AWS RDS.
type RDSDatabaseMigration struct {
	aws                *Client
	masterDBClusterID  *string
	masterInstanceName *string
	replicaClusterID   *string
	replicaDBClusterID *string
}

// NewRDSDatabaseMigration returns a new RDSDatabase interface.
func NewRDSDatabaseMigration(masterInstallationID, replicaClusterID string, awsClient *Client) *RDSDatabaseMigration {
	database := RDSDatabaseMigration{
		aws:                awsClient,
		replicaClusterID:   aws.String(replicaClusterID),
		replicaDBClusterID: aws.String(fmt.Sprintf("%s-migrated", CloudID(masterInstallationID))),
		masterDBClusterID:  aws.String(CloudID(masterInstallationID)),
		masterInstanceName: aws.String(fmt.Sprintf("%s-migrated-master", CloudID(masterInstallationID))),
	}
	return &database
}

// Restore restores database from the most recent snapshot. Optianally, it takes a cluster ID if the
// the intent is to restore the database in another cluster.
func (d *RDSDatabaseMigration) Restore(logger log.FieldLogger) (string, error) {
	vpcs, err := GetVpcsWithFilters([]*ec2.Filter{
		{
			Name:   aws.String(VpcClusterIDTagKey),
			Values: []*string{d.replicaClusterID},
		},
		{
			Name:   aws.String(VpcAvailableTagKey),
			Values: []*string{aws.String(VpcAvailableTagValueFalse)},
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "unabled to restore RDS database")
	}
	if len(vpcs) != 1 {
		return "", errors.Errorf("unabled to restore RDS database: expected 1 VPC in cluster id %s, but got %d", *d.replicaDBClusterID, len(vpcs))
	}

	dbClusterSnapshotsOut, err := d.aws.RDS.DescribeDBClusterSnapshots(&rds.DescribeDBClusterSnapshotsInput{
		SnapshotType: aws.String(RDSDefaultSnapshotType),
	})
	if err != nil {
		return "", errors.Wrap(err, "unabled to restore RDS database")
	}

	expectedTagValue := fmt.Sprintf(DefaultClusterInstallationSnapshotTagValueTemplate, *d.masterDBClusterID)

	var snapshots []*rds.DBClusterSnapshot
	for _, snapshot := range dbClusterSnapshotsOut.DBClusterSnapshots {
		tags, err := d.aws.RDS.ListTagsForResource(&rds.ListTagsForResourceInput{
			ResourceName: snapshot.DBClusterSnapshotArn,
		})
		if err != nil {
			return "", errors.Wrap(err, "unabled to restore RDS database")
		}
		for _, tag := range tags.TagList {
			if tag.Key != nil && tag.Value != nil && *tag.Key == DefaultClusterInstallationSnapshotTagKey &&
				*tag.Value == expectedTagValue {
				snapshots = append(snapshots, snapshot)
			}
		}
	}

	if snapshots == nil {
		return "", errors.Errorf("unabled to restore RDS database: DB cluster %s has no snapshots", *d.masterDBClusterID)
	}

	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotCreateTime.After(*snapshots[j].SnapshotCreateTime)
	})

	if err != nil {
		errors.Wrap(err, "unabled to restore RDS database")
	}

	switch *snapshots[0].Status {
	case RDSStatusCreating:
		logger.Debugf("snapshot of master database is still in progress")
		return model.DatabaseMigrationReplicaCreationIP, nil
	case RDSStatusModifying:
		return "", errors.Errorf("unabled to restore RDS database: snapshot id %s is being modified", *snapshots[0].DBClusterSnapshotIdentifier)
	case RDSStatusDeleting:
		return "", errors.Errorf("unabled to restore RDS database: snapshot id %s is being deleted", *snapshots[0].DBClusterSnapshotIdentifier)
	}

	logger.Debugf("restoring RDS database from snapshot id %s", *snapshots[0].DBClusterSnapshotIdentifier)

	err = d.aws.rdsEnsureRestoreDBClusterFromSnapshot(*vpcs[0].VpcId, *d.replicaDBClusterID, *snapshots[0].DBClusterSnapshotIdentifier, logger)
	if err != nil {
		return "", errors.Wrap(err, "unabled to restore RDS database")
	}
	err = d.aws.rdsEnsureDBClusterInstanceCreated(*d.replicaDBClusterID, *d.masterInstanceName, logger)
	if err != nil {
		return "", errors.Wrap(err, "unabled to restore RDS database")
	}

	logger.WithField("db-cluster-name", *d.replicaDBClusterID).Infof("AWS RDS DB cluster is being restored from %s", *d.masterDBClusterID)

	return model.DatabaseMigrationReplicaCreationComplete, nil
}

// Status returns the status of the database.
func (d *RDSDatabaseMigration) Status(logger log.FieldLogger) (string, error) {
	dbClusterEndpointsOutput, err := d.aws.RDS.DescribeDBClusterEndpoints(&rds.DescribeDBClusterEndpointsInput{
		DBClusterIdentifier: d.replicaDBClusterID,
	})
	if err != nil {
		return "", errors.Wrap(err, "unabled to check RDS database status")
	}
	if len(dbClusterEndpointsOutput.DBClusterEndpoints) < 2 {
		return "", errors.Errorf("unabled to check RDS database status: %s expects at least 2 endpoints", *d.replicaDBClusterID)
	}

	for _, endpoint := range dbClusterEndpointsOutput.DBClusterEndpoints {
		switch *endpoint.Status {
		case RDSStatusCreating:
			return model.DatabaseMigrationReplicaProvisionIP, nil
		case RDSStatusModifying:
			return "", errors.Errorf("unabled to check RDS database status: db cluster id %s is being modified", *d.replicaDBClusterID)
		case RDSStatusDeleting:
			// TODO(gsagula): we should return these status so we can re-use this
			// for teardown operation.
			return "", errors.Errorf("unabled to check RDS database status: db cluster endpoint %s is being deleted", *endpoint.Endpoint)
		}
	}

	dbInstancesOutput, err := d.aws.RDS.DescribeDBInstances(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: d.masterInstanceName,
	})
	if err != nil {
		return "", errors.Wrap(err, "unabled to check RDS database status")
	}
	if len(dbInstancesOutput.DBInstances) < 1 {
		return "", errors.Errorf("unabled to check RDS database status: %s has no instances", *d.replicaDBClusterID)
	}

	for _, instance := range dbInstancesOutput.DBInstances {
		switch *instance.DBInstanceStatus {
		case RDSStatusCreating:
			return model.DatabaseMigrationReplicaProvisionIP, nil
		case RDSStatusModifying:
			return "", errors.Errorf("unabled to check RDS database status: db instance id %s is being modified", *d.masterInstanceName)
		case RDSStatusDeleting:
			// TODO(gsagula): we should return these status so we can re-use this
			// for teardown operation.
			return "", errors.Errorf("unabled to check RDS database status: db instance id %s is being deleted", *d.masterInstanceName)
		}
	}

	return model.DatabaseMigrationReplicaProvisionComplete, nil
}

// Teardown delete any resource created during migration.
func (d *RDSDatabaseMigration) Teardown(logger log.FieldLogger) error {

	// TODO(gsagula): implement it.

	return nil
}
