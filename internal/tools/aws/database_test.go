package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/stretchr/testify/mock"
)

// Acceptance test for provisioning a RDS database.
func (a *AWSTestSuite) TestProvisionRDS() {
	a.SetDescribeVpcsExpectations(a.ClusterA.ID).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &a.VPCa}}}, nil).Once()
	a.SetGetSecretValueExpectations(a.InstallationA.ID).Return(&secretsmanager.GetSecretValueOutput{SecretString: &a.SecretString}, nil).Once()
	a.SetDescribeDBClustersNotFoundExpectation().Once()
	a.SetDescribeSecurityGroupsExpectation().Once()
	a.SetDescribeDBSubnetGroupsExpectation(a.VPCa).Once()
	a.SetCreateDBClusterExpectation(a.InstallationA.ID).Return(nil, nil).Once()

	a.Mocks.LOG.WithFieldArgs("security-group-ids", a.GroupID).Once()
	a.Mocks.LOG.WithFieldString("db-subnet-group-name", SubnetGroupName(a.VPCa)).Once()
	a.Mocks.LOG.WithFieldString("db-cluster-name", CloudID(a.InstallationA.ID)).Once()
	a.Mocks.LOG.WithFieldString("secret-name", RDSSecretName(CloudID(a.InstallationA.ID))).Once()
	a.Mocks.LOG.WithFieldString("db-cluster-name", CloudID(a.InstallationA.ID)).Once()

	a.SetDescribeDBInstancesNotFoundExpectation(a.InstallationA.ID).Once()
	a.SetCreateDBInstanceExpectation(a.InstallationA.ID).Return(nil, nil).Once()
	a.Mocks.LOG.InfofString("Provisioning AWS RDS master instance with name %s", RDSMasterInstanceID(a.InstallationA.ID)).Once()
	a.Mocks.LOG.WithFieldString("db-instance-name", RDSMasterInstanceID(a.InstallationA.ID)).Once()

	err := NewRDSDatabase(a.InstallationA, a.ClusterInstallationA, a.Mocks.AWS).Provision(a.Mocks.LOG.Logger)

	a.Assert().NoError(err)
}

func (a *AWSTestSuite) TestProvisionRDSErrorVPCNotClusterIDProvided() {
	err := NewRDSDatabase(a.InstallationA, nil, a.Mocks.AWS).Provision(a.Mocks.LOG.Logger)
	a.Assert().Error(err)
	a.Assert().Equal("unable to provisioning RDS database - cluster installation id not provided", err.Error())
}

func (a *AWSTestSuite) TestProvisionRDSErrorVPCNotAvailable() {
	a.SetDescribeVpcsExpectations(a.ClusterA.ID).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{}}, nil).Once()

	err := NewRDSDatabase(a.InstallationA, a.ClusterInstallationA, a.Mocks.AWS).Provision(a.Mocks.LOG.Logger)

	a.Assert().Error(err)
	a.Assert().Equal("expected 1 VPC for cluster id000000000000000000000000a, but got 0", err.Error())
}

func (a *AWSTestSuite) TestProvisionRDSTooManyVPCs() {
	a.SetDescribeVpcsExpectations(a.ClusterA.ID).Return(&ec2.DescribeVpcsOutput{
		Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &a.VPCa}, &ec2.Vpc{VpcId: &a.VPCb}},
	}, nil).Once()

	err := NewRDSDatabase(a.InstallationA, a.ClusterInstallationA, a.Mocks.AWS).Provision(a.Mocks.LOG.Logger)

	a.Assert().Error(err)
	a.Assert().Equal("expected 1 VPC for cluster id000000000000000000000000a, but got 2", err.Error())
}

func (a *AWSTestSuite) TestProvisionRDSNotEnoughVPCs() {
	a.SetDescribeVpcsExpectations(a.ClusterA.ID).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{}}, nil).Once()
	a.Mocks.LOG.WithFieldString("secret-name", RDSSecretName(CloudID(a.InstallationA.ID))).Once()

	err := NewRDSDatabase(a.InstallationA, a.ClusterInstallationA, a.Mocks.AWS).Provision(a.Mocks.LOG.Logger)

	a.Assert().Error(err)
	a.Assert().Equal("expected 1 VPC for cluster id000000000000000000000000a, but got 0", err.Error())
}

func (a *AWSTestSuite) TestProvisionRDSErrorMasterUsername() {
	a.SetDescribeVpcsExpectations(a.ClusterA.ID).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &a.VPCa}}}, nil).Once()
	a.SetGetSecretValueExpectations(a.InstallationA.ID).Return(&secretsmanager.GetSecretValueOutput{
		SecretString: &a.SecretStringUserErr,
	}, nil).Once()
	a.Mocks.LOG.WithFieldString("secret-name", RDSSecretName(CloudID(a.InstallationA.ID))).Once()

	err := NewRDSDatabase(a.InstallationA, a.ClusterInstallationA, a.Mocks.AWS).Provision(a.Mocks.LOG.Logger)

	a.Assert().Error(err)
	a.Assert().Equal("RDS master username value is empty", err.Error())
}

func (a *AWSTestSuite) TestProvisionRDSErrorPassword() {
	a.SetDescribeVpcsExpectations(a.ClusterA.ID).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &a.VPCa}}}, nil).Once()
	a.SetGetSecretValueExpectations(a.InstallationA.ID).Return(&secretsmanager.GetSecretValueOutput{
		SecretString: &a.SecretStringPassErr,
	}, nil).Once()
	a.Mocks.LOG.WithFieldString("secret-name", RDSSecretName(CloudID(a.InstallationA.ID))).Once()

	err := NewRDSDatabase(a.InstallationA, a.ClusterInstallationA, a.Mocks.AWS).Provision(a.Mocks.LOG.Logger)

	a.Assert().Error(err)
	a.Assert().Equal("RDS master password value is empty", err.Error())
}

// Helpers

func (a *AWSTestSuite) SetDescribeVpcsExpectations(clusterID string) *mock.Call {
	return a.Mocks.API.EC2.On("DescribeVpcs", mock.MatchedBy(
		func(input *ec2.DescribeVpcsInput) bool {
			return *input.Filters[0].Name == VpcClusterIDTagKey &&
				*input.Filters[1].Name == VpcAvailableTagKey &&
				*input.Filters[0].Values[0] == clusterID &&
				*input.Filters[1].Values[0] == VpcAvailableTagValueFalse
		}))
}

func (a *AWSTestSuite) SetGetSecretValueExpectations(installationID string) *mock.Call {
	return a.Mocks.API.SecretsManager.On("GetSecretValue", mock.MatchedBy(
		func(input *secretsmanager.GetSecretValueInput) bool {
			return *input.SecretId == RDSSecretName(CloudID(installationID))
		}))
}
