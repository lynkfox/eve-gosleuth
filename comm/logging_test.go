package comm_test

import (
	"bytes"
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
)

func TestAddDebugOnlyFields(t *testing.T) {
	// Initialize the logger and set the level to DebugLevel
	logrus.SetLevel(logrus.DebugLevel)

	// Create initial custom fields

	t.Run("When is Debug Level debug only fields are added", func(t *testing.T) {

		comm.InitLogger(logrus.DebugLevel)
		var buf bytes.Buffer
		comm.Logger.SetOutput(&buf)

		fields := comm.LogFields{
			"a": "b",
		}
		fields.AddDebugOnlyFields(logrus.Fields{
			"c": "d",
		})

		comm.Logger.WithFields(logrus.Fields(fields)).Info("debug message with added fields")

		// Verify the first log contains only the first field
		if !containsLog(buf.String(), `"a":"b"`) {
			t.Errorf("Expected field '\"a\":\"b\"' but got: %s", buf.String())
		}

		// Verify the second log contains the second field as well
		if !containsLog(buf.String(), `"c":"d"`) {
			t.Errorf("Expected field '\"a\":\"b\"' but got: %s", buf.String())
		}
	})

	t.Run("When is not debug Level, debug fields are not added", func(t *testing.T) {

		comm.InitLogger(logrus.InfoLevel)
		var buf bytes.Buffer
		comm.Logger.SetOutput(&buf)

		fields := comm.LogFields{
			"a": "b",
		}
		fields.AddDebugOnlyFields(logrus.Fields{
			"c": "d",
		})

		comm.Logger.WithFields(logrus.Fields(fields)).Info("non debug level set should have no additional fields ")

		bufferAsString := buf.String()
		// Verify the first log contains only the first field
		if !containsLog(bufferAsString, `"a":"b"`) {
			t.Errorf("Expected field '\"a\":\"b\"' but got: %s", bufferAsString)
		}

		// Verify the second log contains the second field as well
		if containsLog(bufferAsString, `"c":"d"`) {
			t.Errorf("Debug Only field '\"c\":\"d\"' should not be found but got: %s", bufferAsString)
		}
	})

}

// containsLog checks if the expected substring is present in the log output.
func containsLog(logOutput, expected string) bool {
	return bytes.Contains([]byte(logOutput), []byte(expected))
}
