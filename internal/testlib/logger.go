package testlib

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost-cloud/internal/testlib/mocks"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

// testingWriter is an io.Writer that writes through t.Log
type testingWriter struct {
	tb testing.TB
}

func (tw *testingWriter) Write(b []byte) (int, error) {
	tw.tb.Log(strings.TrimSpace(string(b)))
	return len(b), nil
}

// MakeLogger creates a log.FieldLogger that routes to tb.Log.
func MakeLogger(tb testing.TB) log.FieldLogger {
	logger := log.New()
	logger.SetOutput(&testingWriter{tb})
	logger.SetLevel(log.TraceLevel)

	return logger
}

// MockedFieldLogger supplies a mocked library for testing logs
type MockedFieldLogger struct {
	Logger *mocks.FieldLogger
}

// NewMockedFieldLogger returns a instance of FieldLogger for testing.
func NewMockedFieldLogger() *MockedFieldLogger {
	return &MockedFieldLogger{
		Logger: &mocks.FieldLogger{},
	}
}

// WithFieldArgs set expectations for WithField by passing name and arguments
func (m *MockedFieldLogger) WithFieldArgs(name string, args ...string) *mock.Call {
	return m.Logger.Mock.On("WithField", name, args).Return(logrus.NewEntry(&logrus.Logger{}))
}

// WithFieldString set expectations for WithField by passing name and value
func (m *MockedFieldLogger) WithFieldString(name string, value string) *mock.Call {
	return m.Logger.Mock.On("WithField", name, value).Return(logrus.NewEntry(&logrus.Logger{}))
}

// InfofString set expectations for Infof by passing name and value
func (m *MockedFieldLogger) InfofString(name string, value string) *mock.Call {
	return m.Logger.Mock.On("Infof", name, value).Return(logrus.NewEntry(&logrus.Logger{}))
}

// FieldLogger is a copy of https://github.com/sirupsen/logrus/blob/947831125f318c2fb34bfc6205ee5d74994deb88/logrus.go#L139
// For some reason mokery doesn't generate mocks from the repo.
type FieldLogger interface {
	WithField(key string, value interface{}) *log.Entry
	WithFields(fields log.Fields) *log.Entry
	WithError(err error) *log.Entry

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})

	Debug(args ...interface{})
	Info(args ...interface{})
	Print(args ...interface{})
	Warn(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})

	Debugln(args ...interface{})
	Infoln(args ...interface{})
	Println(args ...interface{})
	Warnln(args ...interface{})
	Warningln(args ...interface{})
	Errorln(args ...interface{})
	Fatalln(args ...interface{})
	Panicln(args ...interface{})
}
