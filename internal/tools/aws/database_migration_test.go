package aws

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type DatabaseMigrationTestSuite struct {
	suite.Suite
	age int
}

func (d *DatabaseMigrationTestSuite) SetupTest() {
	d.age = 5
}

func (d *DatabaseMigrationTestSuite) TestAge() {
	d.Assert().Equal(5, d.age)
}

func TestDatabaseMigrationSuite(t *testing.T) {
	suite.Run(t, new(DatabaseMigrationTestSuite))
}
