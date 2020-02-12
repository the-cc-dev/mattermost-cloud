package aws

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// WARNING:
// This test is meant to exercise the provisioning and teardown of an AWS RDS
// database in a real AWS account. Only set the test env vars below if you wish
// to test this process with real AWS resources.

func TestDatabaseProvision(t *testing.T) {
	id := os.Getenv("SUPER_AWS_DATABASE_TEST")
	if id == "" {
		return
	}

	logger := logrus.New()
	sess, err := CreateSession(logger, SessionConfig{
		Region:  DefaultAWSRegion,
		Retries: 3,
	})
	require.NoError(t, err)

	database := NewRDSDatabase(id, NewClient(sess))
	require.NoError(t, database.Provision(nil, logger))
}

func TestDatabaseTeardown(t *testing.T) {
	id := os.Getenv("SUPER_AWS_DATABASE_TEST")
	if id == "" {
		return
	}

	logger := logrus.New()
	sess, err := CreateSession(logger, SessionConfig{
		Region:  DefaultAWSRegion,
		Retries: 3,
	})
	require.NoError(t, err)

	database := NewRDSDatabase(id, NewClient(sess))
	require.NoError(t, database.Teardown(false, logger))
}
