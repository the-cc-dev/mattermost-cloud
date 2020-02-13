package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/suite"

	"github.com/mattermost/mattermost-cloud/internal/testlib"
	"github.com/mattermost/mattermost-cloud/internal/tools/aws/mocks"
	"github.com/mattermost/mattermost-cloud/model"
)

// ClientTestSuite supplies tests for aws package Client.
type ClientTestSuite struct {
	suite.Suite
	session *session.Session
}

func (d *ClientTestSuite) SetupTest() {
	d.session = session.Must(session.NewSession())
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}

// Holds mocks, clients and fixtures for testing any AWS service. Any new fixtures
// should be added here.
type AWSTestSuite struct {
	suite.Suite

	// Mocked client and services.
	Mocks struct {
		AWS *Client
		API *mocks.AWSMockedServices
		LOG *testlib.MockedFieldLogger
	}

	// General AWS fixtures.
	InstallationA *model.Installation
	InstallationB *model.Installation

	ClusterA *model.Cluster
	ClusterB *model.Cluster

	ClusterInstallationA *model.ClusterInstallation
	ClusterInstallationB *model.ClusterInstallation

	VPCa string
	VPCb string

	// RDS database fixtures.
	RDSSecretID          string
	SecretString         string
	SecretStringUserErr  string
	SecretStringPassErr  string
	DBUser               string
	DBPassword           string
	GroupID              string
	RDSParamGroupCluster string
	RDSParamGroup        string
	DBName               string
	RDSAvailabilityZones []string
}

// This will take care of reseting mocks on every run. Any new mock library should be added here.
func (a *AWSTestSuite) SetupTest() {
	a.Mocks.LOG = testlib.NewMockedFieldLogger()
	a.Mocks.API = mocks.NewAWSMockedServices()
	a.Mocks.AWS = &Client{
		RDS:            a.Mocks.API.RDS,
		EC2:            a.Mocks.API.EC2,
		IAM:            a.Mocks.API.IAM,
		ACM:            a.Mocks.API.ACM,
		S3:             a.Mocks.API.S3,
		Route53:        a.Mocks.API.Route53,
		SecretsManager: a.Mocks.API.SecretsManager,
	}
}

// NewAWSTestSuite gives a new instance of the entire AWS testing suite.
func NewAWSTestSuite() *AWSTestSuite {
	return &AWSTestSuite{
		VPCa: "vpc-000000000000000a",
		VPCb: "vpc-000000000000000b",

		InstallationA: &model.Installation{
			ID: "id000000000000000000000000a",
		},
		InstallationB: &model.Installation{
			ID: "id000000000000000000000000b",
		},

		ClusterA: &model.Cluster{
			ID: "id000000000000000000000000a",
		},
		ClusterB: &model.Cluster{
			ID: "id000000000000000000000000b",
		},

		ClusterInstallationA: &model.ClusterInstallation{
			ID:             "id000000000000000000000000a",
			InstallationID: "id000000000000000000000000a",
			ClusterID:      "id000000000000000000000000a",
		},
		ClusterInstallationB: &model.ClusterInstallation{
			ID:             "id000000000000000000000000b",
			InstallationID: "id000000000000000000000000b",
			ClusterID:      "id000000000000000000000000b",
		},

		DBName:               "mattermost",
		DBUser:               "admin",
		DBPassword:           "secret",
		RDSParamGroupCluster: "mattermost-provisioner-rds-cluster-pg",
		RDSParamGroup:        "mattermost-provisioner-rds-pg",
		RDSAvailabilityZones: []string{"us-east-1a", "us-east-1b", "us-east-1c"},
		GroupID:              "id-0000000000000000",
		SecretString:         "{\"MasterUsername\":\"mmcloud\",\"MasterPassword\":\"oX5rWueZt6ynsijE9PHpUO0VUWSwWSxqXCaZw1dC\"}",
		SecretStringUserErr:  "{\"username\":\"mmcloud\",\"MasterPassword\":\"oX5rWueZt6ynsijE9PHpUO0VUWSwWSxqXCaZw1dC\"}",
		SecretStringPassErr:  "{\"MasterUsername\":\"mmcloud\",\"password\":\"oX5rWueZt6ynsijE9PHpUO0VUWSwWSxqXCaZw1dC\"}",
	}
}

func (a *AWSTestSuite) TestNewClient() {
	session, err := session.NewSession()
	a.Assert().NoError(err)

	client := NewClient(session)

	a.Assert().NotNil(client)
	a.Assert().NotNil(client.ACM)
	a.Assert().NotNil(client.IAM)
	a.Assert().NotNil(client.RDS)
	a.Assert().NotNil(client.S3)
	a.Assert().NotNil(client.Route53)
	a.Assert().NotNil(client.SecretsManager)
}

func TestAWSSuite(t *testing.T) {
	suite.Run(t, NewAWSTestSuite())
}
