package aws

import (
	"testing"
)

var (
	testDNSName             = "example.mattermost.com"
	testParsedHostedZoneID  = "Z3P5QSUBK4POTI"
	testParsedRoute53TagKey = "MattermostCloudDNS"
	testRoute53TagValue     = "public"
)

func TestCreateCNAME(t *testing.T) {
	// tests := []struct {
	// 	name        string
	// 	dnsName     string
	// 	endpoints   []string
	// 	mockError   error
	// 	expectError bool
	// }{
	// 	{
	// 		"no endpoints",
	// 		"dns1",
	// 		[]string{},
	// 		nil,
	// 		true,
	// 	},
	// 	{
	// 		"one endpoints",
	// 		"dns2",
	// 		[]string{"example.mattermost.com"},
	// 		nil,
	// 		false,
	// 	},
	// 	{
	// 		"two endpoints",
	// 		"dns3",
	// 		[]string{"example1.mattermost.com", "example2.mattermost.com"},
	// 		nil,
	// 		false,
	// 	},
	// 	{
	// 		"empty string endpoint",
	// 		"dns4",
	// 		[]string{"example1.mattermost.com", ""},
	// 		nil,
	// 		true,
	// 	},
	// 	{
	// 		"session client error",
	// 		"dns5",
	// 		[]string{"example1.mattermost.com", "example2.mattermost.com"},
	// 		errors.New("mock api error"),
	// 		true,
	// 	},
	// }

	// logger := logrus.New()

	// for _, tt := range tests {
	// 	t.Run(tt.name, func(t *testing.T) {
	// 		a := Client{
	// 			api: &mockAPI{returnedError: tt.mockError},
	// 		}

	// 		err := a.CreatePublicCNAME(tt.dnsName, tt.endpoints, logger)
	// 		switch tt.expectError {
	// 		case true:
	// 			assert.Error(t, err)
	// 		case false:
	// 			assert.NoError(t, err)
	// 		}
	// 	})
	// }
}

func TestDeleteCNAME(t *testing.T) {
	// tests := []struct {
	// 	name          string
	// 	dnsName       string
	// 	mockError     error
	// 	mockTruncated bool
	// 	expectError   bool
	// }{
	// 	{
	// 		"one endpoints, matching",
	// 		testDNSName,
	// 		nil,
	// 		false,
	// 		false,
	// 	}, {
	// 		"two endpoints, no matching",
	// 		"no-matching",
	// 		nil,
	// 		false,
	// 		false,
	// 	}, {
	// 		"session client error",
	// 		"dns4",
	// 		errors.New("mock api error"),
	// 		false,
	// 		true,
	// 	},
	// 	{
	// 		"dns name too long should skip",
	// 		"xoxo-serverwithverylongnametoexposeissuesrelatedtolengthofkeystha",
	// 		nil,
	// 		false,
	// 		false,
	// 	},
	// }

	// logger := logrus.New()

	// for _, tt := range tests {
	// 	t.Run(tt.name, func(t *testing.T) {
	// 		a := Client{
	// 			api: &mockAPI{returnedError: tt.mockError},
	// 		}

	// 		err := a.DeletePublicCNAME(tt.dnsName, logger)
	// 		switch tt.expectError {
	// 		case true:
	// 			assert.Error(t, err)
	// 		case false:
	// 			assert.NoError(t, err)
	// 		}
	// 	})
	// }
}
