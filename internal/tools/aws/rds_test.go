package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/mattermost/mattermost-cloud/internal/testlib"
	"github.com/pkg/errors"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type RDSTestSuite struct {
	suite.Suite
	MockedClient          *MockedClient
	MockedFilledLogger    *testlib.MockedFieldLogger
	ClusterID             string
	ClusterInstallationID string
	InstallationID        string
	DBClusterID           string
	DBClusterInstance     string
	DBUser                string
	DBPassword            string
	GroupID               string
	DBSubnetGroupName     string
	DBPgCluster           string
	DBPg                  string
	VPCID                 string
	DBName                string
	DBAvailabilityZones   []string
}

func NewRDSTestSuite() *RDSTestSuite {
	return &RDSTestSuite{
		VPCID:                 "vpc-0c889fcf75ed9cfbb",
		ClusterID:             "asd876a93nkafgm34maldfl20s",
		InstallationID:        "953qdo7ce7ndjbz3gemrdfff4h",
		ClusterInstallationID: "ajsd83aksk343ksdk34377sdfs",
		DBClusterID:           "cloud-953qdo7ce7ndjbz3gemrdfff4h",
		DBClusterInstance:     "cloud-953qdo7ce7ndjbz3gemrdfff4h-master",
		DBUser:                "admin",
		DBPassword:            "secret",
		GroupID:               "id-0c889fcf75ed9cfbb",
		DBSubnetGroupName:     "mattermost-provisioner-db-vpc-0c889fcf75ed9cfbb",
		DBPgCluster:           "mattermost-provisioner-rds-cluster-pg",
		DBPg:                  "mattermost-provisioner-rds-pg",
		DBName:                "mattermost",
		DBAvailabilityZones:   []string{"us-east-1a", "us-east-1b", "us-east-1c"},
	}
}

func (d *RDSTestSuite) SetupTest() {
	d.MockedClient = NewMockedClient()
	d.MockedFilledLogger = testlib.NewMockedFieldLogger()
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreated() {
	d.SetDescribeDBClustersNotFoundExpectation().Once()
	d.SetDescribeSecurityGroupsExpectation().Once()
	d.SetDescribeDBSubnetGroupsExpectation().Once()
	d.SetCreateDBClusterExpectation().Return(nil, nil).Once()
	d.MockedFilledLogger.WithFieldArgs("security-group-ids", "id-0c889fcf75ed9cfbb").Once()
	d.MockedFilledLogger.WithFieldString("db-subnet-group-name", "mattermost-provisioner-db-vpc-0c889fcf75ed9cfbb").Once()
	d.MockedFilledLogger.WithFieldString("db-cluster-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h").Once()

	err := d.MockedClient.client.rdsEnsureDBClusterCreated(d.DBClusterID, d.VPCID, d.DBUser, d.DBPassword, d.MockedFilledLogger.Logger)

	d.Assert().NoError(err)
	d.MockedClient.api.EC2.AssertExpectations(d.T())
	d.MockedClient.api.RDS.AssertExpectations(d.T())
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedAlreadyCreated() {
	d.SetDescribeDBClustersFoundExpectation().Once()
	d.MockedFilledLogger.WithFieldString("db-cluster-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h").Once()

	err := d.MockedClient.client.rdsEnsureDBClusterCreated(d.DBClusterID, d.VPCID, d.DBUser, d.DBPassword, d.MockedFilledLogger.Logger)

	d.Assert().NoError(err)
	d.MockedClient.api.EC2.AssertNotCalled(d.T(), "DescribeSecurityGroups")
	d.MockedClient.api.RDS.AssertNotCalled(d.T(), "DescribeDBSubnetGroups")
	d.MockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBCluster")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedWithSGError() {
	d.SetDescribeDBClustersNotFoundExpectation().Once()
	d.SetDescribeSecurityGroupsErrorExpectation().Once()

	err := d.MockedClient.client.rdsEnsureDBClusterCreated(d.DBClusterID, d.VPCID, d.DBUser, d.DBPassword, d.MockedFilledLogger.Logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "bad request")
	d.MockedClient.api.RDS.AssertNotCalled(d.T(), "DescribeDBSubnetGroups")
	d.MockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBCluster")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedSubnetError() {
	d.SetDescribeDBClustersNotFoundExpectation().Once()
	d.SetDescribeSecurityGroupsExpectation().Once()
	d.SetDescribeDBSubnetGroupsErrorExpectation().Once()
	d.MockedFilledLogger.WithFieldArgs("security-group-ids", "id-0c889fcf75ed9cfbb").Once()

	err := d.MockedClient.client.rdsEnsureDBClusterCreated(d.DBClusterID, d.VPCID, d.DBUser, d.DBPassword, d.MockedFilledLogger.Logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "bad request")
	d.MockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBCluster")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterCreatedError() {
	d.SetDescribeDBClustersNotFoundExpectation().Once()
	d.SetDescribeSecurityGroupsExpectation().Once()
	d.SetDescribeDBSubnetGroupsExpectation().Once()
	d.SetCreateDBClusterExpectation().Return(nil, nil).Once().Return(nil, errors.New("cannot find parameter groups")).Once()
	d.MockedFilledLogger.WithFieldArgs("security-group-ids", "id-0c889fcf75ed9cfbb").Once()
	d.MockedFilledLogger.WithFieldString("db-subnet-group-name", "mattermost-provisioner-db-vpc-0c889fcf75ed9cfbb").Once()

	err := d.MockedClient.client.rdsEnsureDBClusterCreated(d.DBClusterID, d.VPCID, d.DBUser, d.DBPassword, d.MockedFilledLogger.Logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "cannot find parameter groups")
	d.MockedClient.api.EC2.AssertExpectations(d.T())
	d.MockedClient.api.RDS.AssertExpectations(d.T())
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterInstanceCreated() {
	d.SetDescribeDBInstancesNotFoundExpectation().Once()
	d.SetCreateDBInstanceExpectation().Return(nil, nil)
	d.MockedFilledLogger.InfofString("Provisioning AWS RDS master instance with name %s", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()
	d.MockedFilledLogger.WithFieldString("db-instance-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()

	err := d.MockedClient.client.rdsEnsureDBClusterInstanceCreated(d.DBClusterID, d.DBClusterInstance, d.MockedFilledLogger.Logger)

	d.Assert().NoError(err)
	d.MockedClient.api.RDS.AssertExpectations(d.T())
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterInstanceAlreadyExistError() {
	d.SetDescribeDBInstancesFoundExpectation().Once()
	d.MockedFilledLogger.InfofString("Provisioning AWS RDS master instance with name %s", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()
	d.MockedFilledLogger.WithFieldString("db-instance-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()

	err := d.MockedClient.client.rdsEnsureDBClusterInstanceCreated(d.DBClusterID, d.DBClusterInstance, d.MockedFilledLogger.Logger)

	d.Assert().NoError(err)
	d.MockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBInstance")
}

func (d *RDSTestSuite) TestRDSEnsureDBClusterInstanceCreateError() {
	d.SetDescribeDBInstancesNotFoundExpectation().Once()
	d.SetCreateDBInstanceExpectation().Return(nil, errors.New("bad request"))
	d.MockedFilledLogger.InfofString("Provisioning AWS RDS master instance with name %s", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()
	d.MockedFilledLogger.WithFieldString("db-instance-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()

	err := d.MockedClient.client.rdsEnsureDBClusterInstanceCreated(d.DBClusterID, d.DBClusterInstance, d.MockedFilledLogger.Logger)

	d.Assert().Error(err)
	d.Assert().Equal(err.Error(), "bad request")
	d.MockedClient.api.RDS.AssertNotCalled(d.T(), "CreateDBInstance")
}

func TestRDSSuite(t *testing.T) {
	suite.Run(t, NewRDSTestSuite())
}

// Helpers

func (d *RDSTestSuite) SetCreateDBInstanceExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("CreateDBInstance", mock.MatchedBy(func(input *rds.CreateDBInstanceInput) bool {
		return *input.DBClusterIdentifier == d.DBClusterID &&
			*input.DBParameterGroupName == d.DBPg &&
			*input.DBInstanceIdentifier == d.DBClusterInstance
	}))
}

func (d *RDSTestSuite) SetDescribeDBInstancesNotFoundExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("DescribeDBInstances", mock.MatchedBy(func(input *rds.DescribeDBInstancesInput) bool {
		return *input.DBInstanceIdentifier == d.DBClusterInstance
	})).Return(nil, errors.New("db cluster instance does not exist"))
}

func (d *RDSTestSuite) SetDescribeDBInstancesFoundExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("DescribeDBInstances", mock.MatchedBy(func(input *rds.DescribeDBInstancesInput) bool {
		return *input.DBInstanceIdentifier == d.DBClusterInstance
	})).Return(nil, nil)
}

func (d *RDSTestSuite) SetDescribeDBClustersNotFoundExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("DescribeDBClusters", mock.Anything).Return(nil, errors.New("db cluster does not exist"))
}

func (d *RDSTestSuite) SetDescribeDBClustersFoundExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("DescribeDBClusters", mock.Anything).Return(nil, nil)
}

func (d *RDSTestSuite) SetDescribeSecurityGroupsExpectation() *mock.Call {
	return d.MockedClient.api.EC2.On("DescribeSecurityGroups", mock.AnythingOfType("*ec2.DescribeSecurityGroupsInput")).Return(&ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []*ec2.SecurityGroup{&ec2.SecurityGroup{GroupId: &d.GroupID}},
	}, nil)
}

func (d *RDSTestSuite) SetDescribeSecurityGroupsErrorExpectation() *mock.Call {
	return d.MockedClient.api.EC2.On("DescribeSecurityGroups", mock.AnythingOfType("*ec2.DescribeSecurityGroupsInput")).Return(nil, errors.New("bad request"))
}

func (d *RDSTestSuite) SetDescribeDBSubnetGroupsExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("DescribeDBSubnetGroups", mock.AnythingOfType("*rds.DescribeDBSubnetGroupsInput")).Return(&rds.DescribeDBSubnetGroupsOutput{
		DBSubnetGroups: []*rds.DBSubnetGroup{&rds.DBSubnetGroup{DBSubnetGroupName: &d.DBSubnetGroupName}},
	}, nil)
}

func (d *RDSTestSuite) SetDescribeDBSubnetGroupsErrorExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("DescribeDBSubnetGroups", mock.AnythingOfType("*rds.DescribeDBSubnetGroupsInput")).Return(nil, errors.New("bad request"))
}

func (d *RDSTestSuite) SetCreateDBClusterExpectation() *mock.Call {
	return d.MockedClient.api.RDS.On("CreateDBCluster", mock.MatchedBy(func(input *rds.CreateDBClusterInput) bool {
		for _, zone := range input.AvailabilityZones {
			if !d.Assert().Contains(d.DBAvailabilityZones, *zone) {
				return false
			}
		}
		return *input.BackupRetentionPeriod == 7 &&
			*input.DBClusterIdentifier == d.DBClusterID &&
			*input.DBClusterParameterGroupName == d.DBPgCluster &&
			*input.DBSubnetGroupName == d.DBSubnetGroupName &&
			*input.DatabaseName == d.DBName &&
			*input.VpcSecurityGroupIds[0] == d.GroupID
	}))
}
