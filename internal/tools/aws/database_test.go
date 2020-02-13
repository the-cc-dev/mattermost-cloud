package aws

const RDSSecretString = "{\"MasterUsername\":\"mmcloud\",\"MasterPassword\":\"oX5rWueZt6ynsijE9PHpUO0VUWSwWSxqXCaZw1dC\"}"

// type DatabaseTestSuite struct {
// 	suite.Suite
// 	RDSSecretID         string
// 	SecretString        string
// 	DBClusterID         string
// 	VpcID               string
// 	RDSDatabase         *RDSDatabase
// 	Installation        *model.Installation
// 	ClusterInstallation *model.ClusterInstallation
// 	MockedStore         *modelmock.InstallationDatabaseStoreInterface
// 	RDSTestSuite        *RDSTestSuite
// }

// func NewDatabaseTestSuite() *DatabaseTestSuite {
// 	rdsTestSuite := NewRDSTestSuite()
// 	return &DatabaseTestSuite{
// 		VpcID:        rdsTestSuite.VPCID,
// 		SecretString: RDSSecretString,
// 		RDSSecretID:  RDSSecretName(CloudID(rdsTestSuite.InstallationID)),
// 		DBClusterID:  CloudID(rdsTestSuite.InstallationID),
// 		Installation: &model.Installation{
// 			ID: rdsTestSuite.InstallationID,
// 		},
// 		ClusterInstallation: &model.ClusterInstallation{
// 			ID:             rdsTestSuite.ClusterInstallationID,
// 			ClusterID:      rdsTestSuite.ClusterID,
// 			InstallationID: rdsTestSuite.ClusterInstallationID,
// 		},
// 		RDSTestSuite: rdsTestSuite,
// 	}
// }

// func (d *DatabaseTestSuite) SetupTest() {
// 	d.RDSTestSuite.MockedClient = NewMockedClient()
// 	d.RDSTestSuite.MockedFilledLogger = testlib.NewMockedFieldLogger()
// 	d.RDSDatabase = NewRDSDatabase(d.Installation, d.ClusterInstallation, d.RDSTestSuite.MockedClient.client)
// 	d.MockedStore = &modelmock.InstallationDatabaseStoreInterface{}
// }

// // Acceptance test for provisioning a RDS database.
// func (d *DatabaseTestSuite) TestProvisionRDS() {
// 	d.SetDescribeVpcsExpectations().Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &d.VpcID}}}, nil).Once()
// 	d.SetGetSecretValueExpectations().Return(&secretsmanager.GetSecretValueOutput{SecretString: &d.SecretString}, nil).Once()
// 	d.RDSTestSuite.SetDescribeDBClustersNotFoundExpectation().Once()
// 	d.RDSTestSuite.SetDescribeSecurityGroupsExpectation().Once()
// 	d.RDSTestSuite.SetDescribeDBSubnetGroupsExpectation().Once()
// 	d.RDSTestSuite.SetCreateDBClusterExpectation().Return(nil, nil).Once()
// 	d.RDSTestSuite.MockedFilledLogger.WithFieldArgs("security-group-ids", "id-0c889fcf75ed9cfbb").Once()
// 	d.RDSTestSuite.MockedFilledLogger.WithFieldString("db-subnet-group-name", "mattermost-provisioner-db-vpc-0c889fcf75ed9cfbb").Once()
// 	d.RDSTestSuite.MockedFilledLogger.WithFieldString("secret-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h-rds").Once()
// 	d.RDSTestSuite.MockedFilledLogger.WithFieldString("db-cluster-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h").Once()
// 	d.RDSTestSuite.SetDescribeDBInstancesNotFoundExpectation().Once()
// 	d.RDSTestSuite.SetCreateDBInstanceExpectation().Return(nil, nil).Once()
// 	d.RDSTestSuite.MockedFilledLogger.InfofString("Provisioning AWS RDS master instance with name %s", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()
// 	d.RDSTestSuite.MockedFilledLogger.WithFieldString("db-instance-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h-master").Once()

// 	err := d.RDSDatabase.Provision(d.RDSTestSuite.MockedFilledLogger.Logger)

// 	d.Assert().NoError(err)
// }

// func (d *DatabaseTestSuite) TestProvisionRDSErrorVPCNotClusterIDProvided() {
// 	err := NewRDSDatabase(d.Installation, nil, d.RDSTestSuite.MockedClient.client).Provision(d.RDSTestSuite.MockedFilledLogger.Logger)
// 	d.Assert().Error(err)
// 	d.Assert().Contains(err.Error(), "unable to provisioning RDS database - cluster installation id not provided")
// }

// func (d *DatabaseTestSuite) TestProvisionRDSErrorVPCNotAvailable() {
// 	d.SetDescribeVpcsExpectations().Return(&ec2.DescribeVpcsOutput{}, nil)

// 	err := d.RDSDatabase.Provision(d.RDSTestSuite.MockedFilledLogger.Logger)

// 	d.Assert().Error(err)
// 	d.Assert().Contains(err.Error(), "expected 1 VPC for cluster asd876a93nkafgm34maldfl20s, but got 0")
// }

// func (d *DatabaseTestSuite) TestProvisionRDSTooManyVPCs() {
// 	d.SetDescribeVpcsExpectations().Return(&ec2.DescribeVpcsOutput{
// 		Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &d.VpcID}, &ec2.Vpc{VpcId: aws.String("vpc-123")}},
// 	}, nil).Once()

// 	err := d.RDSDatabase.Provision(d.RDSTestSuite.MockedFilledLogger.Logger)

// 	d.Assert().Error(err)
// 	d.Assert().Contains(err.Error(), "expected 1 VPC for cluster asd876a93nkafgm34maldfl20s, but got 2")
// }

// func (d *DatabaseTestSuite) TestProvisionRDSNotEnoughVPCs() {
// 	d.SetDescribeVpcsExpectations().Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{}}, nil).Once()

// 	err := d.RDSDatabase.Provision(d.RDSTestSuite.MockedFilledLogger.Logger)

// 	d.Assert().Error(err)
// 	d.Assert().Contains(err.Error(), "expected 1 VPC for cluster asd876a93nkafgm34maldfl20s, but got 0")
// }

// func (d *DatabaseTestSuite) TestProvisionRDSErrorMasterUsername() {
// 	d.SetDescribeVpcsExpectations().Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &d.VpcID}}}, nil).Once()
// 	d.SetGetSecretValueExpectations().Return(&secretsmanager.GetSecretValueOutput{
// 		SecretString: aws.String("{\"username\":\"mmcloud\",\"MasterPassword\":\"oX5rWueZt6ynsijE9PHpUO0VUWSwWSxqXCaZw1dC\"}"),
// 	}, nil).Once()
// 	d.RDSTestSuite.MockedFilledLogger.WithFieldString("secret-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h-rds").Once()

// 	err := d.RDSDatabase.Provision(d.RDSTestSuite.MockedFilledLogger.Logger)

// 	d.Assert().Error(err)
// 	d.Assert().Equal("RDS master username value is empty", err.Error())
// }
// func (d *DatabaseTestSuite) TestProvisionRDSErrorMasterPassword() {
// 	d.SetDescribeVpcsExpectations().Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{&ec2.Vpc{VpcId: &d.VpcID}}}, nil).Once()
// 	d.SetGetSecretValueExpectations().Return(&secretsmanager.GetSecretValueOutput{
// 		SecretString: aws.String("{\"MasterUsername\":\"mmcloud\",\"password\":\"oX5rWueZt6ynsijE9PHpUO0VUWSwWSxqXCaZw1dC\"}"),
// 	}, nil).Once()
// 	d.RDSTestSuite.MockedFilledLogger.WithFieldString("secret-name", "cloud-953qdo7ce7ndjbz3gemrdfff4h-rds").Once()

// 	err := d.RDSDatabase.Provision(d.RDSTestSuite.MockedFilledLogger.Logger)

// 	d.Assert().Error(err)
// 	d.Assert().Equal("RDS master password value is empty", err.Error())
// }

// func TestDatabaseSuite(t *testing.T) {
// 	suite.Run(t, NewDatabaseTestSuite())
// }

// // Helpers

// func (d *DatabaseTestSuite) SetDescribeVpcsExpectations() *mock.Call {
// 	return d.RDSTestSuite.MockedClient.api.EC2.On("DescribeVpcs", mock.MatchedBy(
// 		func(input *ec2.DescribeVpcsInput) bool {
// 			return *input.Filters[0].Name == VpcClusterIDTagKey &&
// 				*input.Filters[1].Name == VpcAvailableTagKey &&
// 				*input.Filters[0].Values[0] == d.ClusterInstallation.ClusterID &&
// 				*input.Filters[1].Values[0] == VpcAvailableTagValueFalse
// 		}))
// }

// func (d *DatabaseTestSuite) SetGetSecretValueExpectations() *mock.Call {
// 	return d.RDSTestSuite.MockedClient.api.SecretsManager.On("GetSecretValue", mock.MatchedBy(
// 		func(input *secretsmanager.GetSecretValueInput) bool {
// 			return *input.SecretId == d.RDSSecretID
// 		}))
// }

// // WARNING:
// // This test is meant to exercise the provisioning and teardown of an AWS RDS
// // database in a real AWS account. Only set the test env vars below if you wish
// // to test this process with real AWS resources.

// // func TestDatabaseProvision(t *testing.T) {
// // 	id := os.Getenv("SUPER_AWS_DATABASE_TEST")
// // 	if id == "" {
// // 		return
// // 	}

// // 	logger := logrus.New()
// // 	sess, err := CreateSession(logger, SessionConfig{
// // 		Region:  DefaultAWSRegion,
// // 		Retries: 3,
// // 	})
// // 	require.NoError(t, err)

// // 	database := NewRDSDatabase(&model.Installation{ID: id}, NewClient(sess))
// // 	require.NoError(t, database.Provision(nil, logger))
// // }

// // func TestDatabaseTeardown(t *testing.T) {
// // 	id := os.Getenv("SUPER_AWS_DATABASE_TEST")
// // 	if id == "" {
// // 		return
// // 	}

// // 	logger := logrus.New()
// // 	sess, err := CreateSession(logger, SessionConfig{
// // 		Region:  DefaultAWSRegion,
// // 		Retries: 3,
// // 	})
// // 	require.NoError(t, err)

// // 	database := NewRDSDatabase(id, NewClient(sess))
// // 	require.NoError(t, database.Teardown(false, logger))
// // }
