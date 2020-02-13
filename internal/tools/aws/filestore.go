package aws

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/arn"
	mmv1alpha1 "github.com/mattermost/mattermost-operator/pkg/apis/mattermost/v1alpha1"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const policyARNTemplate = "arn:aws:iam::%s:policy/%s"

// S3Filestore is a filestore backed by AWS S3.
type S3Filestore struct {
	installationID      string
	bucketID            string
	filestoreSecretName string
	awsClient           *Client
}

// NewS3Filestore returns a new S3Filestore interface.
func NewS3Filestore(installationID string, awsClient *Client) *S3Filestore {
	return &S3Filestore{
		installationID:      installationID,
		bucketID:            CloudID(installationID),
		filestoreSecretName: fmt.Sprintf("%s-iam-access-key", CloudID(installationID)),
		awsClient:           awsClient,
	}
}

// Provision completes all the steps necessary to provision an S3 filestore.
func (f *S3Filestore) Provision(logger log.FieldLogger) error {
	err := f.s3FilestoreProvision(f.installationID, logger)
	if err != nil {
		return errors.Wrap(err, "unable to provision AWS S3 filestore")
	}

	return nil
}

// Teardown removes all AWS resources related to an S3 filestore.
func (f *S3Filestore) Teardown(keepData bool, logger log.FieldLogger) error {
	err := f.s3FilestoreTeardown(f.installationID, keepData, logger)
	if err != nil {
		return errors.Wrap(err, "unable to teardown AWS S3 filestore")
	}

	return nil
}

// GenerateFilestoreSpecAndSecret creates the k8s filestore spec and secret for
// accessing the S3 bucket.
func (f *S3Filestore) GenerateFilestoreSpecAndSecret(logger log.FieldLogger) (*mmv1alpha1.Minio, *corev1.Secret, error) {
	iamAccessKey, err := f.awsClient.secretsManagerGetIAMAccessKey(f.bucketID)
	if err != nil {
		return nil, nil, err
	}

	filestoreSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.filestoreSecretName,
		},
		StringData: map[string]string{
			"accesskey": iamAccessKey.ID,
			"secretkey": iamAccessKey.Secret,
		},
	}

	filestoreSpec := &mmv1alpha1.Minio{
		ExternalURL:    S3URL,
		ExternalBucket: f.bucketID,
		Secret:         f.filestoreSecretName,
	}

	logger.Debug("Cluster installation configured to use an AWS S3 filestore")

	return filestoreSpec, filestoreSecret, nil
}

// s3FilestoreProvision provisions an S3 filestore for an installation.
func (f *S3Filestore) s3FilestoreProvision(installationID string, logger log.FieldLogger) error {
	logger.Info("Provisioning AWS S3 filestore")

	user, err := f.awsClient.iamEnsureUserCreated(f.bucketID, logger)
	if err != nil {
		return err
	}

	// The IAM policy lookup requires the AWS account ID for the ARN. The user
	// object contains this ID so we will user that.
	arn, err := arn.Parse(*user.Arn)
	if err != nil {
		return err
	}

	policyARN := fmt.Sprintf(policyARNTemplate, arn.AccountID, f.bucketID)
	policy, err := f.awsClient.iamEnsurePolicyCreated(f.bucketID, policyARN, logger)
	if err != nil {
		return err
	}

	err = f.awsClient.iamEnsurePolicyAttached(f.bucketID, policyARN)
	if err != nil {
		return err
	}

	logger.WithFields(log.Fields{
		"iam-policy-name": *policy.PolicyName,
		"iam-user-name":   *user.UserName,
	}).Debug("AWS IAM policy attached to user")

	err = f.awsClient.s3EnsureBucketCreated(f.bucketID)
	if err != nil {
		return err
	}
	logger.WithField("s3-bucket-name", f.bucketID).Debug("AWS S3 bucket created")

	ak, err := f.awsClient.iamEnsureAccessKeyCreated(f.bucketID, logger)
	if err != nil {
		return err
	}
	logger.WithField("iam-user-name", *user.UserName).Debug("AWS IAM user access key created")

	err = f.awsClient.secretsManagerEnsureIAMAccessKeySecretCreated(f.bucketID, ak)
	if err != nil {
		return err
	}
	logger.WithField("iam-user-name", *user.UserName).Debug("AWS secrets manager secret created")

	return nil
}

func (f *S3Filestore) s3FilestoreTeardown(installationID string, keepData bool, logger log.FieldLogger) error {
	logger.Info("Tearing down AWS S3 filestore")

	err := f.awsClient.iamEnsureUserDeleted(f.bucketID, logger)
	if err != nil {
		return err
	}
	err = f.awsClient.secretsManagerEnsureIAMAccessKeySecretDeleted(f.bucketID, logger)
	if err != nil {
		return err
	}

	if !keepData {
		err = f.awsClient.s3EnsureBucketDeleted(f.bucketID, logger)
		if err != nil {
			return err
		}
		logger.WithField("s3-bucket-name", f.bucketID).Debug("AWS S3 bucket deleted")
	} else {
		logger.WithField("s3-bucket-name", f.bucketID).Info("AWS S3 bucket was left intact due to the keep-data setting of this server")
	}

	return nil
}
