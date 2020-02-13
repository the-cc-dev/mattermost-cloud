package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/mattermost/mattermost-cloud/model"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func (a *Client) rdsGetDBSecurityGroupIDs(vpcID string, logger log.FieldLogger) ([]string, error) {
	result, err := a.EC2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
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
	})
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
	// TODO:
	// The subnet group describe functionality does not currently support
	// filters. Instead, we look up all the subnet groups and match based on
	// name. The name format is based on our terraform creation logic.
	// Example Name: mattermost-provisioner-db-vpc-VPC_ID_HERE
	//
	// We should periodically check if filters become supported and move to that
	// when they do.

	result, err := a.RDS.DescribeDBSubnetGroups(nil)
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
	_, err := a.RDS.DescribeDBClusters(&rds.DescribeDBClustersInput{
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

	input := &rds.CreateDBClusterInput{
		AvailabilityZones:           a.AvailabilityZones,
		BackupRetentionPeriod:       aws.Int64(7),
		DBClusterIdentifier:         aws.String(awsID),
		DatabaseName:                aws.String(RDSDefaultDatabaseName),
		EngineMode:                  aws.String(RDSDefaultEngineMode),
		Engine:                      aws.String(RDSAuroraMySQLEngineName),
		DBClusterParameterGroupName: aws.String(RDSCustomParamGroupClusterName),
		EngineVersion:               aws.String(RDSAuroraDefaultMySQLVersion),
		MasterUserPassword:          aws.String(password),
		MasterUsername:              aws.String(username),
		Port:                        aws.Int64(RDSDefaultMySQLPort),
		StorageEncrypted:            aws.Bool(false),
		DBSubnetGroupName:           aws.String(dbSubnetGroupName),
		VpcSecurityGroupIds:         aws.StringSlice(dbSecurityGroupIDs),
	}

	_, err = a.RDS.CreateDBCluster(input)
	if err != nil {
		return err
	}

	logger.WithField("db-cluster-name", awsID).Debug("AWS DB cluster created")

	return nil
}

func (a *Client) rdsEnsureDBClusterInstanceCreated(awsID, instanceName string, logger log.FieldLogger) error {
	logger.Infof("Provisioning AWS RDS master instance with name %s", instanceName)

	_, err := a.RDS.DescribeDBInstances(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceName),
	})
	if err == nil {
		logger.WithField("db-instance-name", instanceName).Debug("AWS DB instance already created")

		return nil
	}

	_, err = a.RDS.CreateDBInstance(&rds.CreateDBInstanceInput{
		DBClusterIdentifier:  aws.String(awsID),
		DBInstanceIdentifier: aws.String(instanceName),
		DBParameterGroupName: aws.String(RDSCustomParamGroupName),
		DBInstanceClass:      aws.String(RDSDefaultInstanceClass),
		Engine:               aws.String(RDSAuroraMySQLEngineName),
		PubliclyAccessible:   aws.Bool(false),
	})
	if err != nil {
		return err
	}

	logger.WithField("db-instance-name", instanceName).Debug("AWS DB instance created")

	return nil
}

func (a *Client) rdsEnsureDBClusterDeleted(awsID string, logger log.FieldLogger) error {
	result, err := a.RDS.DescribeDBClusters(&rds.DescribeDBClustersInput{
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
		_, err = a.RDS.DeleteDBInstance(&rds.DeleteDBInstanceInput{
			DBInstanceIdentifier: instance.DBInstanceIdentifier,
			SkipFinalSnapshot:    aws.Bool(true),
		})
		if err != nil {
			return errors.Wrap(err, "unable to delete DB cluster instance")
		}
		logger.WithField("db-instance-name", *instance.DBInstanceIdentifier).Debug("DB instance deleted")
	}

	_, err = a.RDS.DeleteDBCluster(&rds.DeleteDBClusterInput{
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
	_, err := a.RDS.CreateDBClusterSnapshot(&rds.CreateDBClusterSnapshotInput{
		DBClusterIdentifier:         aws.String(awsID),
		DBClusterSnapshotIdentifier: aws.String(fmt.Sprintf("%s-snapshot-%s", awsID, model.NewID())),
		Tags:                        tags,
	})
	if err != nil {
		return errors.Wrap(err, "failed to create a DB cluster snapshot for replication")
	}

	return nil
}

func (a *Client) rdsEnsureRestoreDBClusterFromSnapshot(vpcID, awsID, snapshotID string, logger log.FieldLogger) error {
	dbSecurityGroupIDs, err := a.rdsGetDBSecurityGroupIDs(vpcID, logger)
	if err != nil {
		return errors.Wrapf(err, "failed to restore a DB cluster from snapshot in the vpc id: %s", vpcID)
	}

	dbSubnetGroupName, err := a.rdsGetDBSubnetGroupName(vpcID, logger)
	if err != nil {
		return errors.Wrapf(err, "failed to restore a DB cluster from snapshot in the vpc id: %s", vpcID)
	}

	_, err = a.RDS.RestoreDBClusterFromSnapshot(&rds.RestoreDBClusterFromSnapshotInput{
		AvailabilityZones:           a.AvailabilityZones,
		DBClusterIdentifier:         aws.String(awsID),
		DBClusterParameterGroupName: aws.String(RDSCustomParamGroupClusterName),
		DBSubnetGroupName:           aws.String(dbSubnetGroupName),
		DatabaseName:                aws.String(RDSDefaultDatabaseName),
		EngineMode:                  aws.String(RDSDefaultEngineMode),
		Engine:                      aws.String(RDSAuroraMySQLEngineName),
		EngineVersion:               aws.String(RDSAuroraDefaultMySQLVersion),
		Port:                        aws.Int64(RDSDefaultMySQLPort),
		VpcSecurityGroupIds:         aws.StringSlice(dbSecurityGroupIDs),
		SnapshotIdentifier:          aws.String(snapshotID),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to create a DB cluster from snapshot in vpc id: %s", vpcID)
	}

	return nil
}
