package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/sirupsen/logrus"
)

// SessionConfig supplies configuration options for creating an AWS session.
type SessionConfig struct {
	Region  string
	Retries int
}

func (c *SessionConfig) region() *string {
	if c.Region != "" {
		return aws.String(c.Region)
	}
	return nil
}

func (c *SessionConfig) retries() *int {
	if c.Retries > 0 {
		return aws.Int(c.Retries)
	}
	return aws.Int(-1)
}

// CreateSession creates a new AWS session.
func CreateSession(logger log.FieldLogger, config SessionConfig) (*session.Session, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: config.region(),
			// TODO(gsagula): we should supply a Retryer with a more robust strategy or delegate this kind of operations
			// to a sidecar proxy in the future.
			// https://github.com/aws/aws-sdk-go/blob/99cd35c8c7d369ba8c32c46ed306f6c88d24cfd7/aws/request/retryer.go#L20
			MaxRetries: config.retries(),
		},
	})
	if err != nil {
		return nil, err
	}

	sess.Handlers.Send.PushFront(func(r *request.Request) {
		logger.WithField("aws-request", r.ClientInfo.ServiceName).Debugf("%s %s %s", r.HTTPRequest.Method, r.HTTPRequest.URL.String(), r.Params)
	})

	return sess, nil
}
