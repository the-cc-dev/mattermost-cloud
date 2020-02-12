package aws

import (
	"testing"

	"github.com/mattermost/mattermost-cloud/internal/testlib"
	"github.com/mattermost/mattermost-cloud/model"
	modelmocks "github.com/mattermost/mattermost-cloud/model/mocks"
	"github.com/stretchr/testify/suite"
)

type DatabaseTestSuite struct {
	suite.Suite
	RDSTestSuite        *RDSTestSuite
	RDSDatabase         *RDSDatabase
	MockedStore         *modelmocks.InstallationDatabaseStoreInterface
	ClusterInstallation *model.ClusterInstallation
}

func (d *DatabaseTestSuite) SetupTest() {
	d.RDSTestSuite.MockedClient = NewMockedClient()
	d.RDSTestSuite.MockedFilledLogger = testlib.NewMockedFieldLogger()
	d.MockedStore = &modelmocks.InstallationDatabaseStoreInterface{}
	d.RDSDatabase = NewRDSDatabase(d.ClusterInstallation, d.RDSTestSuite.MockedClient.client)
}

func (d *DatabaseTestSuite) TestRestoreErrorVPCNotAvailable() {
	err := d.RDSDatabase.Provision(d.RDSTestSuite.MockedFilledLogger.Logger)
	// d.MockedStore.On("GetClusterInstallations", mock.MatchedBy(func(input *model.ClusterInstallationFilter) bool { return true })).Return([]*model.ClusterInstallation{}).Once()
	d.Assert().Error(err)

	d.Assert().Contains(err.Error(), "unable to lookup cluster installations for installation 953qdo7ce7ndjbz3gemrdfff4h:")
	d.Assert().Contains(err.Error(), "failed to query for clusterInstallations: no such table: ClusterInstallation")
}

func TestDatabaseSuite(t *testing.T) {
	suite.Run(t, &DatabaseTestSuite{
		ClusterInstallation: &model.ClusterInstallation{
			ID:             "264iootcn1ndjbz3ge2rdlprxx",
			ClusterID:      "92je834jfs834js80sksofj343",
			InstallationID: "953qdo7ce7ndjbz3gemrdfff4h",
		},
		RDSTestSuite: &RDSTestSuite{
			VPCID:               "vpc-0c889fcf75ed9cfbb",
			DBClusterID:         "cloud-953qdo7ce7ndjbz3gemrdfff4h",
			DBClusterInstance:   "cloud-953qdo7ce7ndjbz3gemrdfff4h-master",
			DBUser:              "admin",
			DBPassword:          "secret",
			GroupID:             "id-0c889fcf75ed9cfbb",
			DBSubnetGroupName:   "mattermost-provisioner-db-vpc-0c889fcf75ed9cfbb",
			DBPgCluster:         "mattermost-provisioner-rds-cluster-pg",
			DBPg:                "mattermost-provisioner-rds-pg",
			DBName:              "mattermost",
			DBAvailabilityZones: []string{"us-east-1a", "us-east-1b", "us-east-1c"},
		},
	})
}

// WARNING:
// This test is meant to exercise the provisioning and teardown of an AWS RDS
// database in a real AWS account. Only set the test env vars below if you wish
// to test this process with real AWS resources.

// func TestDatabaseProvision(t *testing.T) {
// 	id := os.Getenv("SUPER_AWS_DATABASE_TEST")
// 	if id == "" {
// 		return
// 	}

// 	logger := logrus.New()
// 	sess, err := CreateSession(logger, SessionConfig{
// 		Region:  DefaultAWSRegion,
// 		Retries: 3,
// 	})
// 	require.NoError(t, err)

// 	database := NewRDSDatabase(id, NewClient(sess))
// 	require.NoError(t, database.Provision(nil, logger))
// }

// func TestDatabaseTeardown(t *testing.T) {
// 	id := os.Getenv("SUPER_AWS_DATABASE_TEST")
// 	if id == "" {
// 		return
// 	}

// 	logger := logrus.New()
// 	sess, err := CreateSession(logger, SessionConfig{
// 		Region:  DefaultAWSRegion,
// 		Retries: 3,
// 	})
// 	require.NoError(t, err)

// 	database := NewRDSDatabase(id, NewClient(sess))
// 	require.NoError(t, database.Teardown(false, logger))
// }
