package aws

// type DatabaseMigrationTestSuite struct {
// 	suite.Suite
// 	RDSTestSuite         *RDSTestSuite
// 	RDSDatabaseMigration *RDSDatabaseMigration
// 	MasterInstallationID string
// 	ReplicaClusterID     string
// }

// func (d *DatabaseMigrationTestSuite) SetupTest() {
// 	d.RDSTestSuite.MockedClient = NewMockedClient()
// 	d.RDSDatabaseMigration = NewRDSDatabaseMigration(d.MasterInstallationID, d.ReplicaClusterID, d.RDSTestSuite.MockedClient.client)
// }

// func (d *DatabaseMigrationTestSuite) TestRestore() {
// 	status, err := d.RDSDatabaseMigration.Restore(d.RDSTestSuite.Logger)
// 	d.Assert().Equal("", status)
// 	d.Assert().Error(err)
// 	d.Assert().Equal("unabled to restore RDS database: expected 1 VPC in cluster id cloud-953qdo7ce7ndjbz3gemrdfff4h-migrated, but got 0", err.Error())
// }

// func (d *DatabaseMigrationTestSuite) TestRestoreErrorVPCNotAvailable() {
// 	status, err := d.RDSDatabaseMigration.Restore(d.RDSTestSuite.Logger)
// 	d.Assert().Equal("", status)
// 	d.Assert().Error(err)
// 	d.Assert().Equal("unabled to restore RDS database: expected 1 VPC in cluster id cloud-953qdo7ce7ndjbz3gemrdfff4h-migrated, but got 0", err.Error())
// }

// func TestDatabaseMigrationSuite(t *testing.T) {
// 	suite.Run(t, &DatabaseMigrationTestSuite{
// 		MasterInstallationID: "953qdo7ce7ndjbz3gemrdfff4h",
// 		ReplicaClusterID:     "125qgh7cy7ndard5gemrdnop4h",
// 		RDSTestSuite: &RDSTestSuite{
// 			Logger:              logger.New(),
// 			VPCID:               "vpc-0c889fcf75ed9cfbb",
// 			DBClusterID:         "cloud-953qdo7ce7ndjbz3gemrdfff4h",
// 			DBClusterInstance:   "cloud-953qdo7ce7ndjbz3gemrdfff4h-master",
// 			DBUser:              "admin",
// 			DBPassword:          "secret",
// 			GroupID:             "id-0c889fcf75ed9cfbb",
// 			DBSubnetGroupName:   "mattermost-provisioner-db-vpc-0c889fcf75ed9cfbb",
// 			DBPgCluster:         "mattermost-provisioner-rds-cluster-pg",
// 			DBPg:                "mattermost-provisioner-rds-pg",
// 			DBName:              "mattermost",
// 			DBAvailabilityZones: []string{"us-east-1a", "us-east-1b", "us-east-1c"},
// 		},
// 	})
// }
