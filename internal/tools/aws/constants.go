package aws

const (
	// S3URL is the S3 URL for making bucket API calls.
	S3URL = "s3.amazonaws.com"

	// DefaultAWSRegion is the default AWS region for AWS resources.
	DefaultAWSRegion = "us-east-1"

	// VpcAvailableTagKey is the tag key to determine if a VPC is currently in
	// use by a cluster or not.
	VpcAvailableTagKey = "tag:Available"

	// VpcAvailableTagValueTrue is the tag value for VpcAvailableTagKey when the
	// VPC is currently not in use by a cluster and can be claimed.
	VpcAvailableTagValueTrue = "true"

	// VpcAvailableTagValueFalse is the tag value for VpcAvailableTagKey when the
	// VPC is currently in use by a cluster and cannot be claimed.
	VpcAvailableTagValueFalse = "false"

	// VpcClusterIDTagKey is the tag key used to store the cluster ID of the
	// cluster running in that VPC.
	VpcClusterIDTagKey = "tag:CloudClusterID"

	// VpcClusterOwnerKey is the tag key  used to store the owner of the
	// cluster's human name so that the VPC's owner can be identified
	VpcClusterOwnerKey = "tag:CloudClusterOwner"

	// VpcClusterOwnerValueNone is the tag value for VpcClusterOwnerKey when
	// there is no cluster running in the VPC.
	VpcClusterOwnerValueNone = "none"

	// VpcClusterIDTagValueNone is the tag value for VpcClusterIDTagKey when
	// there is no cluster running in the VPC.
	VpcClusterIDTagValueNone = "none"

	// DefaultDBSubnetGroupName is the default DB subnet group name used when
	// creating DB clusters. This group name is defined by the owner of the AWS
	// accounts and can be the same across all accounts.
	// Note: This needs to be manually created before RDS databases can be used.
	DefaultDBSubnetGroupName = "mattermost-databases"

	// DefaultDBSecurityGroupTagKey is the default DB security group tag key
	// that is used to find security groups to use in configuration of the RDS
	// database.
	// Note: This needs to be manually created before RDS databases can be used.
	DefaultDBSecurityGroupTagKey = "tag:MattermostCloudInstallationDatabase"

	// DefaultDBSecurityGroupTagValue is the default DB security group tag value
	// that is used to find security groups to use in configuration of the RDS
	// database.
	// Note: This needs to be manually created before RDS databases can be used.
	DefaultDBSecurityGroupTagValue = "MYSQL/Aurora"

	// DefaultDBSubnetGroupTagKey is the default DB subnet group tag key that is
	// used to find subnet groups to use in configuration of the RDS database.
	// Note: This needs to be manually created before RDS databases can be used.
	DefaultDBSubnetGroupTagKey = "tag:MattermostCloudInstallationDatabase"

	// DefaultDBSubnetGroupTagValue is the default DB subnet group tag value
	// that is used to find subnet groups to use in configuration of the RDS
	// database.
	// Note: This needs to be manually created before RDS databases can be used.
	DefaultDBSubnetGroupTagValue = "MYSQL/Aurora"

	// DefaultInstallCertificatesTagKey is the default key used to find the server
	// TLS certificate ARN.
	DefaultInstallCertificatesTagKey = "tag:MattermostCloudInstallationCertificates"

	// DefaultInstallCertificatesTagValue is the default value used to find the server
	// TLS certificate ARN.
	DefaultInstallCertificatesTagValue = "true"

	// DefaultCloudDNSTagKey is the default key used to find private and public hosted
	// zone IDs in AWS Route53.
	DefaultCloudDNSTagKey = "tag:MattermostCloudDNS"

	// DefaultPrivateCloudDNSTagValue is the default value used to find private hosted zone ID
	// in AWS Route53.
	DefaultPrivateCloudDNSTagValue = "private"

	// DefaultPublicCloudDNSTagValue is the default value used to find public hosted zone ID
	// in AWS Route53.
	DefaultPublicCloudDNSTagValue = "public"

	// DefaultClusterInstallationSnapshotTagKey is used for tagging snapshots of a cluster installation.
	DefaultClusterInstallationSnapshotTagKey = "tag:ClusterInstallationSnapshot"

	// DefaultClusterInstallationSnapshotTagValueTemplate is used for tagging snapshots of a cluster installation.
	// Cluster installation id should be passed to this template when tagging the resource.
	DefaultClusterInstallationSnapshotTagValueTemplate = "rds-snapshot-%s"

	// RDSAuroraMySQLEngineName ..
	RDSAuroraMySQLEngineName = "aurora-mysql"

	// RDSAuroraDefaultMySQLVersion ..
	RDSAuroraDefaultMySQLVersion = "5.7"

	// RDSCustomParamGroupClusterName ..
	RDSCustomParamGroupClusterName = "mattermost-provisioner-rds-cluster-pg"

	// RDSCustomParamGroupName ...
	RDSCustomParamGroupName = "mattermost-provisioner-rds-pg"

	// RDSDefaultInstanceClass ..
	RDSDefaultInstanceClass = "db.r5.large"

	// RDSDefaultEngineMode ..
	RDSDefaultEngineMode = "provisioned"

	// RDSDefaultDatabaseName ..
	RDSDefaultDatabaseName = "mattermost"

	// RDSDefaultSnapshotType ..
	RDSDefaultSnapshotType = "manual"

	// RDSStatusAvailable ..
	RDSStatusAvailable = "available"

	// RDSStatusDeleting ..
	RDSStatusDeleting = "deleting"

	// RDSStatusCreating ..
	RDSStatusCreating = "creating"

	// RDSStatusModifying ..
	RDSStatusModifying = "modifying"

	// RDSDefaultMySQLPort ..
	RDSDefaultMySQLPort = 3306

	// cloudIDPrefix is the prefix value used when creating AWS resource names.
	// Warning:
	// changing this value will break the connection to AWS resources for
	// existing installations.
	cloudIDPrefix = "cloud-"

	// iamSuffix is the suffix value used when referencing an AWS IAM secret.
	// Warning:
	// changing this value will break the connection to AWS resources for
	// existing installations.
	iamSuffix = "-iam"

	// rdsSuffix is the suffix value used when referencing an AWS RDS secret.
	// Warning:
	// changing this value will break the connection to AWS resources for
	// existing installations.
	rdsSuffix = "-rds"
)
