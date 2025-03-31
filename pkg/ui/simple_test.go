package ui

import (
	"sync"
	"testing"
)

func TestIsRecording(t *testing.T) {
	s := &Simple{}

	// Initially not recording
	s.isRecording = false
	if s.IsRecording() {
		t.Error("Expected IsRecording() to return false initially")
	}

	// Set to recording state
	s.isRecording = true
	if !s.IsRecording() {
		t.Error("Expected IsRecording() to return true after setting isRecording")
	}
}

func TestCallbacks(t *testing.T) {
	s := &Simple{
		mu: sync.Mutex{},
	}

	var recordCalled, stopCalled bool

	s.SetCallbacks(
		func() { recordCalled = true },
		func() { stopCalled = true },
	)

	// Test callbacks are set and can be called
	if s.onRecord == nil {
		t.Error("Expected onRecord callback to be set")
	}
	if s.onStop == nil {
		t.Error("Expected onStop callback to be set")
	}

	// Call the callbacks
	s.onRecord()
	s.onStop()

	if !recordCalled {
		t.Error("Expected record callback to be called")
	}
	if !stopCalled {
		t.Error("Expected stop callback to be called")
	}
}
