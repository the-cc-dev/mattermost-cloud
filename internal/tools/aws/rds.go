package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/mattermost/mattermost-cloud/model"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	AuroraMySQLEngineName    = "aurora-mysql"
	AuroraMySQLEngineVersion = "5.7"
	RDSCustomParamGroupName  = "replication-aurora-mysql57"
	RDSDefaultInstanceClass  = "db.r5.large"
	RDSDefaultEngineMode     = "provisioned"
	RDSDefaultDatabaseName   = "mattermost"
	RDSDefaultSnapshotType   = "manual"
	RDSStatusAvailable       = "available"
	RDSStatusDeleting        = "deleting"
	RDSStatusCreating        = "creating"
	RDSStatusModifying       = "modifying"
	RDSDefaultMySQLPort      = 3306
)

// DBClusterSnapshot holds the information about the snapshot of a RDS database cluster.
type DBClusterSnapshot struct {
	SnapshotID string
	Status     string
}

func (a *Client) rdsGetDBSecurityGroupIDs(vpcID string, logger log.FieldLogger) ([]string, error) {
	svc := ec2.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcID)},
			},
			{
				Name:   aws.String(DefaultDBSecurityGroupTagKey),
				Values: []*string{aws.String(DefaultDBSecurityGroupTagValue)},
			},
		},
	}

	result, err := svc.DescribeSecurityGroups(input)
	if err != nil {
		return []string{}, err
	}

	var dbSecurityGroups []string
	for _, sg := range result.SecurityGroups {
		dbSecurityGroups = append(dbSecurityGroups, *sg.GroupId)
	}

	if len(dbSecurityGroups) == 0 {
		return []string{}, fmt.Errorf("unable to find security groups tagged for Mattermost DB usage: %s=%s", DefaultDBSecurityGroupTagKey, DefaultDBSecurityGroupTagValue)
	}

	logger.WithField("security-group-ids", dbSecurityGroups).Debugf("Found %d DB tagged security groups", len(dbSecurityGroups))

	return dbSecurityGroups, nil
}

func (a *Client) rdsGetDBSubnetGroupName(vpcID string, logger log.FieldLogger) (string, error) {
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	// TODO:
	// The subnet group describe functionality does not currently support
	// filters. Instead, we look up all the subnet groups and match based on
	// name. The name format is based on our terraform creation logic.
	// Example Name: mattermost-provisioner-db-vpc-VPC_ID_HERE
	//
	// We should periodically check if filters become supported and move to that
	// when they do.

	result, err := svc.DescribeDBSubnetGroups(nil)
	if err != nil {
		return "", err
	}

	for _, subnetGroup := range result.DBSubnetGroups {
		// AWS names are unique, so there will only be one that correctly matches.
		if *subnetGroup.DBSubnetGroupName == fmt.Sprintf("mattermost-provisioner-db-%s", vpcID) {
			name := *subnetGroup.DBSubnetGroupName
			logger.WithField("db-subnet-group-name", name).Debugf("Found DB subnet group")

			return name, nil
		}
	}

	return "", fmt.Errorf("unable to find subnet group tagged for Mattermost DB usage: %s=%s", DefaultDBSubnetGroupTagKey, DefaultDBSubnetGroupTagValue)
}

func (a *Client) rdsEnsureDBClusterCreated(awsID, vpcID, username, password string, logger log.FieldLogger) error {
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	_, err := svc.DescribeDBClusters(&rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(awsID),
	})
	if err == nil {
		logger.WithField("db-cluster-name", awsID).Debug("AWS DB cluster already created")

		return nil
	}

	dbSecurityGroupIDs, err := a.rdsGetDBSecurityGroupIDs(vpcID, logger)
	if err != nil {
		return err
	}

	dbSubnetGroupName, err := a.rdsGetDBSubnetGroupName(vpcID, logger)
	if err != nil {
		return err
	}

	rdsTags := []*rds.Tag{&rds.Tag{
		Key:   aws.String(""),
		Value: aws.String(""),
	}}

	input := &rds.CreateDBClusterInput{
		AvailabilityZones: []*string{
			aws.String("us-east-1a"),
			aws.String("us-east-1b"),
			aws.String("us-east-1c"),
		},
		BackupRetentionPeriod:       aws.Int64(7),
		DBClusterIdentifier:         aws.String(awsID),
		DatabaseName:                aws.String(RDSDefaultDatabaseName),
		EngineMode:                  aws.String(RDSDefaultEngineMode),
		Engine:                      aws.String(AuroraMySQLEngineName),
		DBClusterParameterGroupName: aws.String(RDSCustomParamGroupName),
		EngineVersion:               aws.String(AuroraMySQLEngineVersion),
		MasterUserPassword:          aws.String(password),
		MasterUsername:              aws.String(username),
		Port:                        aws.Int64(RDSDefaultMySQLPort),
		StorageEncrypted:            aws.Bool(false),
		DBSubnetGroupName:           aws.String(dbSubnetGroupName),
		VpcSecurityGroupIds:         aws.StringSlice(dbSecurityGroupIDs),
		Tags:                        rdsTags,
	}

	_, err = svc.CreateDBCluster(input)
	if err != nil {
		return err
	}

	logger.WithField("db-cluster-name", awsID).Debug("AWS DB cluster created")

	return nil
}

func (a *Client) rdsEnsureDBClusterInstanceCreated(awsID, instanceName string, logger log.FieldLogger) error {
	logger.Infof("Provisioning AWS RDS master instance with name %s", instanceName)
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	_, err := svc.DescribeDBInstances(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceName),
	})
	if err == nil {
		logger.WithField("db-instance-name", instanceName).Debug("AWS DB instance already created")

		return nil
	}

	_, err = svc.CreateDBInstance(&rds.CreateDBInstanceInput{
		DBClusterIdentifier:  aws.String(awsID),
		DBInstanceIdentifier: aws.String(instanceName),
		DBParameterGroupName: aws.String(RDSCustomParamGroupName),
		DBInstanceClass:      aws.String(RDSDefaultInstanceClass),
		Engine:               aws.String(AuroraMySQLEngineName),
		PubliclyAccessible:   aws.Bool(false),
	})
	if err != nil {
		return err
	}

	logger.WithField("db-instance-name", instanceName).Debug("AWS DB instance created")

	return nil
}

func rdsGetDBCluster(awsID string, logger log.FieldLogger) (*rds.DBCluster, error) {
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	result, err := svc.DescribeDBClusters(&rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(awsID),
	})
	if err != nil {
		return nil, err
	}

	if len(result.DBClusters) != 1 {
		return nil, fmt.Errorf("expected 1 DB cluster, but got %d", len(result.DBClusters))
	}

	return result.DBClusters[0], nil
}

func (a *Client) rdsEnsureDBClusterDeleted(awsID string, logger log.FieldLogger) error {
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	result, err := svc.DescribeDBClusters(&rds.DescribeDBClustersInput{
		DBClusterIdentifier: aws.String(awsID),
	})
	if err != nil {
		return err
	}

	if len(result.DBClusters) != 1 {
		return fmt.Errorf("expected 1 DB cluster, but got %d", len(result.DBClusters))
	}

	cluster := result.DBClusters[0]

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == rds.ErrCodeDBClusterNotFoundFault {
				logger.WithField("db-cluster-name", awsID).Warn("DBCluster could not be found; assuming already deleted")
				return nil
			}
		}
		return err
	}

	for _, instance := range cluster.DBClusterMembers {
		_, err = svc.DeleteDBInstance(&rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: instance.DBInstanceIdentifier,
			SkipFinalSnapshot:    aws.Bool(true),
		})
		if err != nil {
			return errors.Wrap(err, "unable to delete DB cluster instance")
		}
		logger.WithField("db-instance-name", *instance.DBInstanceIdentifier).Debug("DB instance deleted")
	}

	_, err = svc.DeleteDBCluster(&rds.DeleteDBClusterInput{
		DBClusterIdentifier: aws.String(awsID),
		SkipFinalSnapshot:   aws.Bool(true),
	})
	if err != nil {
		return errors.Wrap(err, "unable to delete DB cluster")
	}

	logger.WithField("db-cluster-name", awsID).Debug("DBCluster deleted")

	return nil
}

func (a *Client) rdsEnsureDBClusterSnapshotCreated(awsID string, tags []*rds.Tag) error {
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	_, err := svc.CreateDBClusterSnapshot(&rds.CreateDBClusterSnapshotInput{
		DBClusterIdentifier:         aws.String(awsID),
		DBClusterSnapshotIdentifier: aws.String(fmt.Sprintf("%s-snapshot-%s", awsID, model.NewID())),
		Tags:                        tags,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create a DB cluster snapshot for replication")
	}

	return nil
}

func (a *Client) rdsGetSnapshotTagsMap(filter []*rds.Filter) (*map[*rds.DBClusterSnapshot][]*rds.Tag, error) {
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	dbClusterSnapshotsOut, err := svc.DescribeDBClusterSnapshots(&rds.DescribeDBClusterSnapshotsInput{
		SnapshotType: aws.String(RDSDefaultSnapshotType),
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe RDS database cluster")
	}

	snapshotTagListMap := make(map[*rds.DBClusterSnapshot][]*rds.Tag, len(dbClusterSnapshotsOut.DBClusterSnapshots))
	for _, snapshot := range dbClusterSnapshotsOut.DBClusterSnapshots {
		listTagsForResourceOut, err := svc.ListTagsForResource(&rds.ListTagsForResourceInput{
			ResourceName: snapshot.DBClusterSnapshotArn,
			Filters:      filter,
		})
		if err != nil {
			return nil, err
		}
		snapshotTagListMap[snapshot] = listTagsForResourceOut.TagList
	}

	return &snapshotTagListMap, nil
}

// func (a *Client) rdsGetDBClusterSnapshot(key, value string) (*rds.DBClusterSnapshot, error) {
// 	svc := rds.New(session.New(), &aws.Config{
// 		Region: aws.String(DefaultAWSRegion),
// 	})

// 	dbClusterSnapshotsOut, err := svc.DescribeDBClusterSnapshots(&rds.DescribeDBClusterSnapshotsInput{
// 		SnapshotType: aws.String(RDSDefaultSnapshotType),
// 	})
// 	if err != nil {
// 		return nil, errors.Wrap(err, "failed to describe RDS database cluster")
// 	}

// 	var snapshots []*rds.DBClusterSnapshot
// 	for _, snapshot := range dbClusterSnapshotsOut.DBClusterSnapshots {
// 		listTagsForResourceOut, err := svc.ListTagsForResource(&rds.ListTagsForResourceInput{
// 			ResourceName: snapshot.DBClusterSnapshotArn,
// 		})
// 		if err != nil {
// 			return nil, err
// 		}
// 		for _, tag := range listTagsForResourceOut.TagList {
// 			if *tag.Key == key {
// 				if tag.Value != nil && *tag.Value == value {
// 					snapshots = append(snapshots, snapshot)
// 				}
// 			}
// 		}
// 	}

// 	length := len(snapshots)
// 	if length != 1 {
// 		return nil, errors.Errorf("only one snapshot should be associated with tag %s:%s, but found %v", key, value, length)
// 	}

// 	return snapshots[0], nil
// }

func (a *Client) rdsEnsureRestoreDBClusterFromSnapshot(vpcID, awsID, snapshotID string, logger log.FieldLogger) error {
	svc := rds.New(session.New(), &aws.Config{
		Region: aws.String(DefaultAWSRegion),
	})

	dbSecurityGroupIDs, err := a.rdsGetDBSecurityGroupIDs(vpcID, logger)
	if err != nil {
		return errors.Wrapf(err, "failed to restore a DB cluster from snapshot in the vpc id: %s", vpcID)
	}

	dbSubnetGroupName, err := a.rdsGetDBSubnetGroupName(vpcID, logger)
	if err != nil {
		return errors.Wrapf(err, "failed to restore a DB cluster from snapshot in the vpc id: %s", vpcID)
	}

	_, err = svc.RestoreDBClusterFromSnapshot(&rds.RestoreDBClusterFromSnapshotInput{
		AvailabilityZones: []*string{
			aws.String("us-east-1a"),
			aws.String("us-east-1b"),
			aws.String("us-east-1c"),
		},
		DBClusterIdentifier:         aws.String(awsID),
		DBClusterParameterGroupName: aws.String(RDSCustomParamGroupName),
		DBSubnetGroupName:           aws.String(dbSubnetGroupName),
		DatabaseName:                aws.String(RDSDefaultDatabaseName),
		EngineMode:                  aws.String(RDSDefaultEngineMode),
		Engine:                      aws.String(AuroraMySQLEngineName),
		EngineVersion:               aws.String(AuroraMySQLEngineVersion),
		Port:                        aws.Int64(RDSDefaultMySQLPort),
		VpcSecurityGroupIds:         aws.StringSlice(dbSecurityGroupIDs),
		SnapshotIdentifier:          aws.String(snapshotID),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to create a DB cluster from snapshot in vpc id: %s", vpcID)
	}

	return nil
}
