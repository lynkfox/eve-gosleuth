package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNewUser(t *testing.T) {
	t.Run("reports the flag startup set", func(t *testing.T) {
		assert.True(t, (&App{newUser: true}).IsNewUser())
		assert.False(t, (&App{}).IsNewUser())
	})

	t.Run("waits for startup", func(t *testing.T) {
		app := NewApp("1.1.0")
		answer := make(chan bool)
		go func() { answer <- app.IsNewUser() }()

		app.newUser = true
		app.markReady()
		assert.True(t, <-answer)
	})
}
