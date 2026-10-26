package comm_test

import (
	"testing"

	"github.com/lynkfox/go_eve_sleuth/comm"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestReportProgress(t *testing.T) {
	comm.InitLogger(logrus.DebugLevel)

	t.Run("forwards formatted messages to the reporter", func(t *testing.T) {
		received := []string{}
		comm.SetProgressReporter(func(message string) {
			received = append(received, message)
		})
		defer comm.SetProgressReporter(nil)

		comm.ReportProgress("Fetching killmail details from ESI: %d/%d", 3, 11)
		comm.ReportProgress("Ranking fleetmates")

		assert.Equal(t, []string{"Fetching killmail details from ESI: 3/11", "Ranking fleetmates"}, received)
	})

	t.Run("no reporter set does not panic", func(t *testing.T) {
		comm.SetProgressReporter(nil)

		assert.NotPanics(t, func() { comm.ReportProgress("Ranking fleetmates") })
	})
}
