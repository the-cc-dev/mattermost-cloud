package mocks

// AWSMockedServices supplies a AWS mocked services api.
type AWSMockedServices struct {
	RDS            *RDSAPI
	ACM            *ACMAPI
	EC2            *EC2API
	S3             *S3API
	IAM            *IAMAPI
	Route53        *Route53API
	SecretsManager *SecretsManagerAPI
}

// NewAWSMockedServices returns a new instance of AWSMockedServices.
func NewAWSMockedServices() *AWSMockedServices {
	return &AWSMockedServices{
		RDS:            new(RDSAPI),
		EC2:            new(EC2API),
		ACM:            new(ACMAPI),
		S3:             new(S3API),
		IAM:            new(IAMAPI),
		Route53:        new(Route53API),
		SecretsManager: new(SecretsManagerAPI),
	}
}
