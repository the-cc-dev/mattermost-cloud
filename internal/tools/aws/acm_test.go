package aws

import (
	"testing"
)

var (
	testARNCertificate          = "arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"
	testParsedCertificateTagKey = "MattermostCloudInstallationCertificates"
	testCertificateTagValue     = "true"
)

func TestGetCertificateByTag(t *testing.T) {
	// a := Client{api: &mockAPI{}}
	// list, err := a.GetCertificateSummaryByTag(testParsedCertificateTagKey, testCertificateTagValue)
	// assert.NoError(t, err)
	// assert.Equal(t, *list.CertificateArn, testARNCertificate)
}

func TestGetCertificateByTagError(t *testing.T) {
	// a := Client{api: &mockAPI{returnedError: errors.New("something went wrong")}}
	// _, err := a.GetCertificateSummaryByTag(testParsedRoute53TagKey, testRoute53TagValue)
	// assert.Error(t, err)
}

func TestGetCertificateByTagWrongKey(t *testing.T) {
	// a := Client{api: &mockAPI{}}
	// _, err := a.GetCertificateSummaryByTag("banana", testRoute53TagValue)
	// assert.Error(t, err)
}

func TestGetCertificateByTagEmptyValue(t *testing.T) {
	// a := Client{api: &mockAPI{}}
	// _, err := a.GetCertificateSummaryByTag(testParsedRoute53TagKey, "")
	// assert.Error(t, err)
}
