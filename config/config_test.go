package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test hotkey defaults
	if !cfg.HotKeyCtrl {
		t.Error("Expected default HotKeyCtrl to be true")
	}
	if !cfg.HotKeyShift {
		t.Error("Expected default HotKeyShift to be true")
	}
	if cfg.HotKeyAlt {
		t.Error("Expected default HotKeyAlt to be false")
	}
	if cfg.HotKeyKey != "s" {
		t.Errorf("Expected default HotKeyKey to be 's', got '%s'", cfg.HotKeyKey)
	}

	// Test audio defaults
	if cfg.AudioSampleRate != 16000 {
		t.Errorf("Expected default AudioSampleRate to be 16000, got %d", cfg.AudioSampleRate)
	}
	if cfg.AudioBufferSize != 1024 {
		t.Errorf("Expected default AudioBufferSize to be 1024, got %d", cfg.AudioBufferSize)
	}
	if cfg.AudioChannels != 1 {
		t.Errorf("Expected default AudioChannels to be 1, got %d", cfg.AudioChannels)
	}

	// Test Whisper defaults - get expected model path
	homeDir, err := os.UserHomeDir()
	if err == nil {
		expectedModelPath := filepath.Join(homeDir, ".ramble", "models")
		if cfg.WhisperModelPath != expectedModelPath {
			t.Errorf("Expected default WhisperModelPath to be '%s', got '%s'", expectedModelPath, cfg.WhisperModelPath)
		}
	}
	if cfg.WhisperModelType != "tiny" {
		t.Errorf("Expected default WhisperModelType to be 'tiny', got '%s'", cfg.WhisperModelType)
	}

	// Test UI defaults
	if !cfg.ShowTranscriptionUI {
		t.Error("Expected default ShowTranscriptionUI to be true")
	}
	if !cfg.InsertTextAtCursor {
		t.Error("Expected default InsertTextAtCursor to be true")
	}
}

func TestCurrentConfig(t *testing.T) {
	// Test that Current is initialized with default values
	if Current == nil {
		t.Fatal("Current config should not be nil")
	}

	// Verify a few values
	if Current.HotKeyKey != "s" {
		t.Errorf("Expected Current.HotKeyKey to be 's', got '%s'", Current.HotKeyKey)
	}
	if Current.AudioSampleRate != 16000 {
		t.Errorf("Expected Current.AudioSampleRate to be 16000, got %d", Current.AudioSampleRate)
	}
}
