package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type RDSTestSuite struct {
	suite.Suite
	mockedClient      *MockedClient
	logger            log.FieldLogger
	vpcID             string
	dbClusterID       string
	dbClusterInstance string
	dbUser            string
	dbPassword        string
	groupID           string
	dbSubnetGroupName string
	dbPGCluster       string
	dbPG              string
	dbName            string
	dbZones           []string
}

func (d *RDSTestSuite) SetupTest() {
	d.mockedClient = NewMockedClient()
	d.logger = log.New()
	d.vpcID = "vpc-0c889fcf75ed9cfbb"
	d.dbClusterID = "cloud-953qdo7ce7ndjbz3gemrdfff4h"
	d.dbClusterInstance = fmt.Sprintf("%s-master", d.dbClusterID)
	d.dbUser = "admin"
	d.dbPassword = "secret"
	d.groupID = "id-0c889fcf75ed9cfbb"
	d.dbSubnetGroupName = fmt.Sprintf("mattermost-provisioner-db-%s", d.vpcID)
	d.dbPGCluster = "mattermost-provisioner-rds-cluster-pg"
	d.dbPG = "mattermost-provisioner-rds-pg"
	d.dbName = "mattermost"
	d.dbZones = []string{"us-east-1a", "us-east-1b", "us-east-1c"}
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreated() {
	d.mockedClient.api.RDS.On("DescribeDBClusters", mock.Anything).Return(nil, errors.New("db cluster does not exist")).Once()
	d.mockedClient.api.EC2.On("DescribeSecurityGroups", mock.AnythingOfType("*ec2.DescribeSecurityGroupsInput")).Return(&ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []*ec2.SecurityGroup{&ec2.SecurityGroup{GroupId: &d.groupID}},
	}, nil).Once()
	d.mockedClient.api.RDS.On("DescribeDBSubnetGroups", mock.AnythingOfType("*rds.DescribeDBSubnetGroupsInput")).Return(&rds.DescribeDBSubnetGroupsOutput{
		DBSubnetGroups: []*rds.DBSubnetGroup{&rds.DBSubnetGroup{DBSubnetGroupName: &d.dbSubnetGroupName}},
	}, nil).Once()
	d.mockedClient.api.RDS.On("CreateDBCluster", mock.MatchedBy(func(input *rds.CreateDBClusterInput) bool {
		for _, zone := range input.AvailabilityZones {
			if !d.Assert().Contains(d.dbZones, *zone) {
				return false
			}
		}
		return *input.BackupRetentionPeriod == 7 &&
			*input.DBClusterIdentifier == d.dbClusterID &&
			*input.DBClusterParameterGroupName == d.dbPGCluster &&
			*input.DBSubnetGroupName == d.dbSubnetGroupName &&
			*input.DatabaseName == d.dbName &&
			*input.VpcSecurityGroupIds[0] == d.groupID
	})).Return(nil, nil).Once()

	err := d.mockedClient.client.rdsEnsureDBClusterCreated(d.dbClusterID, d.vpcID, d.dbUser, d.dbPassword, d.logger)

	d.Assert().NoError(err)
	d.mockedClient.api.EC2.AssertExpectations(d.T())
	d.mockedClient.api.RDS.AssertExpectations(d.T())
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedAlreadyCreated() {
	d.mockedClient.api.RDS.On("DescribeDBClusters", mock.Anything).Return(nil, nil).Once()
	err := d.mockedClient.client.rdsEnsureDBClusterCreated(d.dbClusterID, d.vpcID, d.dbUser, d.dbPassword, d.logger)

	d.Assert().NoError(err)
	d.mockedClient.api.EC2.AssertNotCalled(d.T(), "DescribeSecurityGroups")
	d.mockedClient.api.RDS.AssertNotCalled(d.T(), "DescribeDBSubnetGroups")
	d.mockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBCluster")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedWithSGError() {
	d.mockedClient.api.RDS.On("DescribeDBClusters", mock.Anything).Return(nil, errors.New("db cluster already exist")).Once()
	d.mockedClient.api.EC2.On("DescribeSecurityGroups", mock.AnythingOfType("*ec2.DescribeSecurityGroupsInput")).Return(nil, errors.New("bad request")).Once()

	err := d.mockedClient.client.rdsEnsureDBClusterCreated(d.dbClusterID, d.vpcID, d.dbUser, d.dbPassword, d.logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "bad request")
	d.mockedClient.api.RDS.AssertNotCalled(d.T(), "DescribeDBSubnetGroups")
	d.mockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBCluster")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedSubnetError() {
	d.mockedClient.api.RDS.On("DescribeDBClusters", mock.Anything).Return(nil, errors.New("db cluster already exist")).Once()
	d.mockedClient.api.EC2.On("DescribeSecurityGroups", mock.AnythingOfType("*ec2.DescribeSecurityGroupsInput")).Return(&ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []*ec2.SecurityGroup{&ec2.SecurityGroup{GroupId: &d.groupID}},
	}, nil).Once()
	d.mockedClient.api.RDS.On("DescribeDBSubnetGroups", mock.AnythingOfType("*rds.DescribeDBSubnetGroupsInput")).Return(&rds.DescribeDBSubnetGroupsOutput{
		DBSubnetGroups: []*rds.DBSubnetGroup{&rds.DBSubnetGroup{DBSubnetGroupName: &d.dbSubnetGroupName}},
	}, errors.New("bad request")).Once()

	err := d.mockedClient.client.rdsEnsureDBClusterCreated(d.dbClusterID, d.vpcID, d.dbUser, d.dbPassword, d.logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "bad request")
	d.mockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBCluster")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedError() {
	d.mockedClient.api.RDS.On("DescribeDBClusters", mock.Anything).Return(nil, errors.New("db cluster already exist")).Once()
	d.mockedClient.api.EC2.On("DescribeSecurityGroups", mock.AnythingOfType("*ec2.DescribeSecurityGroupsInput")).Return(&ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []*ec2.SecurityGroup{&ec2.SecurityGroup{GroupId: &d.groupID}},
	}, nil).Once()
	d.mockedClient.api.RDS.On("DescribeDBSubnetGroups", mock.AnythingOfType("*rds.DescribeDBSubnetGroupsInput")).Return(&rds.DescribeDBSubnetGroupsOutput{
		DBSubnetGroups: []*rds.DBSubnetGroup{&rds.DBSubnetGroup{DBSubnetGroupName: &d.dbSubnetGroupName}},
	}, nil).Once()
	d.mockedClient.api.RDS.On("CreateDBCluster", mock.MatchedBy(func(input *rds.CreateDBClusterInput) bool {
		for _, zone := range input.AvailabilityZones {
			if !d.Assert().Contains(d.dbZones, *zone) {
				return false
			}
		}
		return *input.BackupRetentionPeriod == 7 &&
			*input.DBClusterIdentifier == d.dbClusterID &&
			*input.DBClusterParameterGroupName == d.dbPGCluster &&
			*input.DBSubnetGroupName == d.dbSubnetGroupName &&
			*input.DatabaseName == d.dbName &&
			*input.VpcSecurityGroupIds[0] == d.groupID
	})).Return(nil, errors.New("bad request")).Once()

	err := d.mockedClient.client.rdsEnsureDBClusterCreated(d.dbClusterID, d.vpcID, d.dbUser, d.dbPassword, d.logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "bad request")
	d.mockedClient.api.EC2.AssertExpectations(d.T())
	d.mockedClient.api.RDS.AssertExpectations(d.T())
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterInstanceCreated() {
	d.mockedClient.api.RDS.On("DescribeDBInstances", mock.MatchedBy(func(input *rds.DescribeDBInstancesInput) bool {
		return *input.DBInstanceIdentifier == d.dbClusterInstance
	})).Return(nil, errors.New("db cluster instance does not exist")).Once()

	d.mockedClient.api.RDS.On("CreateDBInstance", mock.MatchedBy(func(input *rds.CreateDBInstanceInput) bool {
		return *input.DBClusterIdentifier == d.dbClusterID &&
			*input.DBParameterGroupName == d.dbPG &&
			*input.DBInstanceIdentifier == d.dbClusterInstance
	})).Return(nil, nil)

	err := d.mockedClient.client.rdsEnsureDBClusterInstanceCreated(d.dbClusterID, d.dbClusterInstance, d.logger)

	d.Assert().NoError(err)
	d.mockedClient.api.RDS.AssertExpectations(d.T())
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterInstanceAlreadyExistError() {
	d.mockedClient.api.RDS.On("DescribeDBInstances", mock.MatchedBy(func(input *rds.DescribeDBInstancesInput) bool {
		return *input.DBInstanceIdentifier == d.dbClusterInstance
	})).Return(nil, nil).Once()

	err := d.mockedClient.client.rdsEnsureDBClusterInstanceCreated(d.dbClusterID, d.dbClusterInstance, d.logger)

	d.Assert().NoError(err)
	d.mockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBInstance")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterInstanceCreateError() {
	d.mockedClient.api.RDS.On("DescribeDBInstances", mock.MatchedBy(func(input *rds.DescribeDBInstancesInput) bool {
		return *input.DBInstanceIdentifier == d.dbClusterInstance
	})).Return(nil, errors.New("db cluster instance does not exist")).Once()

	d.mockedClient.api.RDS.On("CreateDBInstance", mock.MatchedBy(func(input *rds.CreateDBInstanceInput) bool {
		return *input.DBClusterIdentifier == d.dbClusterID &&
			*input.DBParameterGroupName == d.dbPG &&
			*input.DBInstanceIdentifier == d.dbClusterInstance
	})).Return(nil, errors.New("bad request"))

	err := d.mockedClient.client.rdsEnsureDBClusterInstanceCreated(d.dbClusterID, d.dbClusterInstance, d.logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "bad request")
	d.mockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBInstance")
}

func TestRDSSuite(t *testing.T) {
	suite.Run(t, new(RDSTestSuite))
}
