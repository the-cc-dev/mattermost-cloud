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
func NewRDSDatabaseMigration(masterInstallationID, replicaClusterID string) *RDSDatabaseMigration {
	database := RDSDatabaseMigration{
		replicaClusterID:   aws.String(replicaClusterID),
		replicaDBClusterID: aws.String(fmt.Sprintf("%s-migrated", CloudID(masterInstallationID))),
		masterDBClusterID:  aws.String(CloudID(masterInstallationID)),
		masterInstanceName: aws.String(fmt.Sprintf("%s-migrated-master", CloudID(masterInstallationID))),

		// TODO(gsagula): change this after refactoring tools/aws.
		aws: New(),
	}
	return &database
}

func (d *RDSDatabaseMigration) getMostRecentSnapshot() (*rds.DBClusterSnapshot, error) {
	dbClusterSnapshotsOut, err := d.aws.describeDBClusterSnapshots(&rds.DescribeDBClusterSnapshotsInput{
		SnapshotType: aws.String(RDSDefaultSnapshotType),
	})
	if err != nil {
		return nil, errors.Wrap(err, "unabled to restore RDS database")
	}

	expectedTagValue := fmt.Sprintf(DefaultClusterInstallationSnapshotTagValueTemplate, *d.masterDBClusterID)

	var snapshots []*rds.DBClusterSnapshot
	for _, snapshot := range dbClusterSnapshotsOut.DBClusterSnapshots {
		tags, err := d.aws.listTagsForResource(&rds.ListTagsForResourceInput{
			ResourceName: snapshot.DBClusterSnapshotArn,
		})
		if err != nil {
			return nil, errors.Wrap(err, "unabled to restore RDS database")
		}
		for _, tag := range tags.TagList {
			if tag.Key != nil && tag.Value != nil && *tag.Key == DefaultClusterInstallationSnapshotTagKey &&
				*tag.Value == expectedTagValue {
				snapshots = append(snapshots, snapshot)
			}
		}
	}

	if snapshots == nil {
		return nil, errors.Errorf("unabled to restore RDS database: DB cluster %s has no snapshots", *d.masterDBClusterID)
	}

	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotCreateTime.After(*snapshots[j].SnapshotCreateTime)
	})

	return snapshots[0], nil
}

// Restore restores database from the most recent snapshot. Optianally, it takes a cluster ID if the
// the intent is to restore the database in another cluster.
func (d *RDSDatabaseMigration) Restore(logger log.FieldLogger) error {
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
		return errors.Wrap(err, "unabled to restore RDS database")
	}
	if len(vpcs) != 1 {
		return errors.Errorf("unabled to restore RDS database: expected 1 VPC in cluster id %s, but got %d", *d.replicaDBClusterID, len(vpcs))
	}

	snapshot, err := d.getMostRecentSnapshot()
	if err != nil {
		errors.Wrap(err, "unabled to restore RDS database")
	}
	if *snapshot.Status != RDSStatusAvailable {
		return errors.New("unabled to restore RDS database - snapshot is not available")
	}

	logger.Debugf("restoring RDS database from snapshot id %s", snapshot.DBClusterSnapshotIdentifier)

	err = d.aws.rdsEnsureRestoreDBClusterFromSnapshot(*vpcs[0].VpcId, *d.replicaDBClusterID, *snapshot.DBClusterSnapshotIdentifier, logger)
	if err != nil {
		return errors.Wrap(err, "unabled to restore RDS database")
	}
	err = d.aws.rdsEnsureDBClusterInstanceCreated(*d.replicaDBClusterID, *d.masterInstanceName, logger)
	if err != nil {
		return errors.Wrap(err, "unabled to restore RDS database")
	}

	logger.WithField("db-cluster-name", *d.replicaDBClusterID).Infof("AWS RDS DB cluster is being restored from %s", *d.masterDBClusterID)

	return nil
}

// Snapshot takes a snapshot of the RDS database.
func (d *RDSDatabaseMigration) Snapshot(logger log.FieldLogger) error {
	err := d.aws.rdsEnsureDBClusterSnapshotCreated(*d.masterDBClusterID, []*rds.Tag{&rds.Tag{
		Key:   aws.String(DefaultClusterInstallationSnapshotTagKey),
		Value: aws.String(fmt.Sprintf(DefaultClusterInstallationSnapshotTagValueTemplate, *d.masterDBClusterID)),
	}})
	if err != nil {
		return errors.Wrapf(err, "unabled to snapshot RDS database: %s", *d.masterDBClusterID)
	}

	logger.WithField("db-cluster-name", *d.masterDBClusterID).Info("RDS database snapshot in progress")

	return nil
}

// SnapshotStatus returns the status of the most recent snapshot.
func (d *RDSDatabaseMigration) SnapshotStatus(logger log.FieldLogger) (string, error) {
	snapshot, err := d.getMostRecentSnapshot()
	if err != nil {
		return "", errors.Wrapf(err, "unabled to restore RDS database")
	}

	switch *snapshot.Status {
	case RDSStatusAvailable:
		logger.WithField("db-cluster-name", *d.masterDBClusterID).Info("RDS database snapshot creation completed")
	case RDSStatusCreating:
		return model.DatabaseMigrationSnapshotCreationIP, nil
	case RDSStatusModifying:
		return model.DatabaseMigrationSnapshotModifying, nil
	case RDSStatusDeleting:
		return "", errors.Errorf("unabled to restore RDS database: snapshot id %s is being deleted", *snapshot.DBClusterSnapshotIdentifier)
	default:
		return "", errors.Errorf("unknown snapshot status %s", *snapshot.Status)
	}

	return model.DatabaseMigrationSnapshotCreationComplete, nil
}

// DatabaseStatus returns the status of the database.
func (d *RDSDatabaseMigration) DatabaseStatus(logger log.FieldLogger) (string, error) {
	input := rds.DescribeDBClusterEndpointsInput{
		DBClusterIdentifier: d.replicaDBClusterID,
	}

	dbClusterEndpointsOutput, err := d.aws.describeDBClusterEndpoints(&input)
	if err != nil {
		return "", errors.Wrap(err, "unabled to check RDS database status")
	}
	if len(dbClusterEndpointsOutput.DBClusterEndpoints) < 1 {
		return "", errors.Errorf("unabled to restore RDS database: %s has no endpoints", *d.replicaDBClusterID)
	}

	for _, endpoint := range dbClusterEndpointsOutput.DBClusterEndpoints {
		switch *endpoint.Status {
		case RDSStatusAvailable:
			logger.WithField("db-cluster-name", *d.replicaDBClusterID).Info("RDS db cluster endpoints are ready")
		case RDSStatusCreating:
			return model.DatabaseMigrationDatabaseCreationIP, nil
		case RDSStatusModifying:
			return "", errors.Errorf("unabled to restore RDS database: db cluster id %s is being modified", *d.replicaDBClusterID)
		case RDSStatusDeleting:
			return "", errors.Errorf("unabled to restore RDS database: endpoint %s is being deleted", *endpoint.Endpoint)
		default:
			return "", errors.Errorf("unabled to restore RDS database: unknown endpoint status %s", *endpoint.Status)
		}
	}

	//  describeDBInstancesEndpoints()

	return model.DatabaseMigrationDatabaseCreationComplete, nil
}
