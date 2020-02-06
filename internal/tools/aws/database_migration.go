package aws

import (
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	awstools "github.com/mattermost/mattermost-cloud/internal/tools/aws"
	"github.com/mattermost/mattermost-cloud/model"
)

// RDSDatabaseMigration is a migrated database backed by AWS RDS.
type RDSDatabaseMigration struct {
	aws                *Client
	masterDBClusterID  *string
	masterInstanceID   *string
	replicaDBClusterID *string
	logger             log.FieldLogger
}

// NewRDSDatabaseMigration returns a new RDSDatabase interface.
func NewRDSDatabaseMigration(masterInstallationID, replicaClusterID string, logger log.FieldLogger, awsClient awstools.AWS) *RDSDatabaseMigration {
	database := RDSDatabaseMigration{
		replicaDBClusterID: aws.String(fmt.Sprintf("%s-migrated", CloudID(masterInstallationID))),
		masterInstanceID:   aws.String(fmt.Sprintf("%s-master", masterInstallationID)),
		masterDBClusterID:  aws.String(CloudID(masterInstallationID)),
		aws:                awsClient,
		logger:             logger,
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
		return nil, errors.Errorf("unabled to restore RDS database: DB cluster %s has no snapshots", *d.masterDBClusterID)
	}

	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].SnapshotCreateTime.After(*snapshots[j].SnapshotCreateTime)
	})

	return snapshots[0], nil
}

// Restore restores database from the most recent snapshot. Optianally, it takes a cluster ID if the
// the intent is to restore the database in another cluster.
func (d *RDSDatabaseMigration) Restore() error {
	vpcs, err := GetVpcsWithFilters([]*ec2.Filter{
		{
			Name:   aws.String(VpcClusterIDTagKey),
			Values: []*string{d.replicaDBClusterID},
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

	err = d.aws.rdsEnsureRestoreDBClusterFromSnapshot(*vpcs[0].VpcId, *d.replicaDBClusterID, *snapshot.DBClusterSnapshotIdentifier, d.logger)
	if err != nil {
		return errors.Wrap(err, "unabled to restore RDS database")
	}
	err = d.aws.rdsEnsureDBClusterInstanceCreated(*d.replicaDBClusterID, fmt.Sprintf("%s-master", *d.masterDBClusterID), d.logger)
	if err != nil {
		return errors.Wrap(err, "unabled to restore RDS database")
	}

	return nil
}

// Snapshot takes a snapshot of the RDS database.
func (d *RDSDatabaseMigration) Snapshot() error {
	err := d.aws.rdsEnsureDBClusterSnapshotCreated(*d.replicaDBClusterID, []*rds.Tag{&rds.Tag{
		Key:   aws.String(DefaultClusterInstallationSnapshotTagKey),
		Value: aws.String(fmt.Sprintf(DefaultClusterInstallationSnapshotTagValueTemplate, *d.replicaDBClusterID)),
	}})
	if err != nil {
		return errors.Wrapf(err, "unabled to snapshot RDS database: %s", *d.replicaDBClusterID)
	}

	return nil
}

// SnapshotStatus returns the status of the most recent snapshot.
func (d *RDSDatabaseMigration) SnapshotStatus() (string, error) {
	snapshot, err := d.getMostRecentSnapshot()
	if err != nil {
		return "", err
	}

	switch *snapshot.Status {
	case RDSStatusCreating:
		return model.DatabaseMigrationSnapshotStatusIP, nil

	case RDSStatusModifying:
		return model.DatabaseMigrationSnapshotStatusIP, nil

	case RDSStatusDeleting:
		return model.DatabaseMigrationSnapshotStatusIP, errors.Errorf("unabled to restore RDS database: snapshot id %s is being deleted", *snapshots[0].DBClusterSnapshotIdentifier)

	default:
		return "", errors.Errorf("unabled to restore RDS database: unknown snapshot status %s", *snapshot.Status)
	}

	return model.DatabaseMigrationSnapshotStatusReady, nil
}

// DatabaseStatus returns the status of the database.
func (d *RDSDatabaseMigration) DatabaseStatus() (string, error) {
	input := rds.DescribeDBClusterEndpointsInput{
		DBClusterIdentifier: d.replicaDBClusterID,
	}

	output, err := d.aws.describeDBClusterEndpoints(&input)
	if err != nil {
		return "", errors.Wrap(err, "unabled to check RDS database status")
	}
	if len(output.DBClusterEndpoints) < 1 {
		return "", errors.Errorf("unabled to restore RDS database: %s has no endpoints", *d.replicaDBClusterID)
	}

	for _, endpoint := range output.DBClusterEndpoints {
		fmt.Println(endpoint)
		switch *endpoint.Status {
		case RDSStatusDeleting:
			return model.DatabaseMigrationRestoreFailing, nil
		case RDSStatusCreating:
			return model.DatabaseMigrationRestoreInProgress, nil
		case RDSStatusModifying:
			return model.DatabaseMigrationRestoreInProgress, nil
		}
	}

	return model.DatabaseMigrationRestoreComplete, nil
}
