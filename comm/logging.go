package comm

import (
	"maps"
	"os"

	"github.com/sirupsen/logrus"
)

var Logger *logrus.Logger

// Custom type that wraps logrus.LogFields
type LogFields logrus.Fields

func InitLogger(level logrus.Level) {
	Logger = logrus.New()
	Logger.SetFormatter(&logrus.JSONFormatter{})
	Logger.SetOutput(os.Stdout)
	Logger.SetLevel(level)
}

// AddDebugOnlyFields adds the fields to the CustomFields map only if the current log level is DebugLevel.
func (f *LogFields) AddDebugOnlyFields(newFields logrus.Fields) {
	if Logger.GetLevel() == logrus.DebugLevel {
		maps.Copy(*f, newFields)
	}
}

func (f *LogFields) AddTemporaryDebugOnlyFields(newFields logrus.Fields) logrus.Fields {
	combinedFields := logrus.Fields{}
	maps.Copy(combinedFields, *f)
	if Logger.GetLevel() == logrus.DebugLevel {
		maps.Copy(combinedFields, newFields)
	}
	return combinedFields
}
