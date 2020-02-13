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

func (d *ClientTestSuite) TestNewClient() {
	client := NewClient(d.session)

	d.Assert().NotNil(client)
	d.Assert().NotNil(client.RDS)
	d.Assert().NotNil(client.S3)
	d.Assert().NotNil(client.IAM)
	d.Assert().NotNil(client.ACM)
	d.Assert().NotNil(client.SecretsManager)
	d.Assert().NotNil(client.Route53)
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientTestSuite))
}

// MockedClient supplies a AWS mocks and mocked AWS client.
type MockedClient struct {
	api    *mocks.AWSMockedServices
	client *Client
}

// NewMockedClient returns a instance of a mocked AWS client.
func NewMockedClient() *MockedClient {
	mockedClient := &MockedClient{
		api: mocks.NewAWSMockedServices(),
	}
	mockedClient.client = &Client{
		RDS:            mockedClient.api.RDS,
		EC2:            mockedClient.api.EC2,
		IAM:            mockedClient.api.IAM,
		ACM:            mockedClient.api.ACM,
		S3:             mockedClient.api.S3,
		Route53:        mockedClient.api.Route53,
		SecretsManager: mockedClient.api.SecretsManager,
	}

	return mockedClient
}

// Holds mocks, clients and fixtures for testing any AWS service. Any new fixtures
// should be added here.
type AWSTestSuite struct {
	suite.Suite
	Mocks struct {
		AWS *Client
		API *mocks.AWSMockedServices
		LOG *testlib.MockedFieldLogger
	}

	InstallationA *model.Installation
	InstallationB *model.Installation

	ClusterA *model.Cluster
	ClusterB *model.Cluster

	ClusterInstallationA *model.ClusterInstallation
	ClusterInstallationB *model.ClusterInstallation

	VPCa string
	VPCb string

	// Database Test
	RDSSecretID  string
	SecretString string

	Installation        *model.Installation
	ClusterInstallation *model.ClusterInstallation

	// RDS Test
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
	}
}
