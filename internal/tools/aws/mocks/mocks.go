package mocks

// Mocks supplies helper functions for AWS mocked client.
type Mocks struct {
	RDS     *RDSAPI
	ACM     *ACMAPI
	EC2     *EC2API
	S3      *S3API
	IAM     *IAMAPI
	Route53 *Route53API
}

// NewMocks returns a new mocked AWS client.
func NewMocks() *Mocks {
	return &Mocks{
		RDS:     new(RDSAPI),
		EC2:     new(EC2API),
		ACM:     new(ACMAPI),
		S3:      new(S3API),
		IAM:     new(IAMAPI),
		Route53: new(Route53API),
	}
}
