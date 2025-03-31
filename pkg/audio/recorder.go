// Package audio provides functionality for capturing audio input
package audio

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/gordonklaus/portaudio"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// Configuration for audio recording
type Config struct {
	// Sample rate in Hz (e.g., 16000)
	SampleRate float64
	// Number of channels (1 for mono, 2 for stereo)
	Channels int
	// Buffer size in frames
	FramesPerBuffer int
	// Debug mode for verbose logging
	Debug bool
}

// DefaultConfig returns a reasonable default configuration for speech recognition
func DefaultConfig() Config {
	return Config{
		SampleRate:      16000, // 16kHz is good for speech
		Channels:        1,     // Mono for speech recognition
		FramesPerBuffer: 1024,  // Reasonable buffer size
		Debug:           false,
	}
}

// Recorder handles audio capture from the microphone
type Recorder struct {
	config       Config
	stream       *portaudio.Stream
	buffer       []float32
	isRecording  bool
	dataCallback func([]float32)
	mu           sync.Mutex
	initialized  bool
}

// NewRecorder creates a new audio recorder with the given configuration
func NewRecorder(config Config) (*Recorder, error) {
	recorder := &Recorder{
		config:      config,
		buffer:      make([]float32, config.FramesPerBuffer*config.Channels),
		isRecording: false,
		initialized: false,
	}

	// Initialize PortAudio with explicit error handling
	err := portaudio.Initialize()
	if err != nil {
		if config.Debug {
			logger.Error(logger.CategoryAudio, "PortAudio initialization error: %v", err)

			// Check for common ALSA errors
			errMsg := err.Error()
			if strings.Contains(errMsg, "ALSA") {
				logger.Warning(logger.CategoryAudio, "ALSA error detected. This is usually due to a configuration issue.")
				logger.Info(logger.CategoryAudio, "- Check if ALSA is properly installed")
				logger.Info(logger.CategoryAudio, "- Try running 'aplay -l' to list audio devices")
				logger.Info(logger.CategoryAudio, "- You may need to configure ~/.asoundrc or /etc/asound.conf")
			}
		}
		return nil, fmt.Errorf("failed to initialize PortAudio: %w", err)
	}

	recorder.initialized = true

	// Print available devices if in debug mode
	if config.Debug {
		devices, err := portaudio.Devices()
		if err != nil {
			logger.Error(logger.CategoryAudio, "Error getting audio devices: %v", err)
		} else {
			logger.Info(logger.CategoryAudio, "Available audio devices:")
			for i, dev := range devices {
				logger.Info(logger.CategoryAudio, "[%d] %s (in: %v, out: %v)", i, dev.Name, dev.MaxInputChannels > 0, dev.MaxOutputChannels > 0)
			}
		}

		// Print PortAudio version information
		versionText := portaudio.VersionText()
		logger.Info(logger.CategoryAudio, "PortAudio version: %s", versionText)
	}

	return recorder, nil
}

// Start begins audio recording
// The provided callback will be called with audio data
func (r *Recorder) Start(callback func([]float32)) error {
	r.mu.Lock()

	if r.isRecording {
		r.mu.Unlock()
		return errors.New("recorder is already running")
	}

	// Store the callback
	r.dataCallback = callback

	// Debug logging - no need to hold the lock for this
	if r.config.Debug {
		r.mu.Unlock() // Release lock during potentially slow API calls

		defaultHostApi, err := portaudio.DefaultHostApi()
		if err == nil {
			logger.Info(logger.CategoryAudio, "Using audio API: %s", defaultHostApi.Name)
			if defaultHostApi.DefaultInputDevice != nil {
				logger.Info(logger.CategoryAudio, "Default input device: %s", defaultHostApi.DefaultInputDevice.Name)
			} else {
				logger.Warning(logger.CategoryAudio, "Warning: No default input device found")
			}
		} else {
			logger.Warning(logger.CategoryAudio, "Could not get default host API: %v", err)
		}

		r.mu.Lock() // Reacquire lock
	}

	// Try to open the default audio stream - this can block, so release lock
	r.mu.Unlock()

	// Just use the default stream which is most compatible
	stream, err := portaudio.OpenDefaultStream(
		r.config.Channels, // Input channels
		0,                 // No output channels
		r.config.SampleRate,
		r.config.FramesPerBuffer,
		r.processAudio,
	)

	r.mu.Lock() // Reacquire lock

	if err != nil {
		if r.config.Debug {
			logger.Error(logger.CategoryAudio, "Failed to open audio stream: %v", err)

			// Provide guidance for common error cases
			errMsg := err.Error()
			if strings.Contains(errMsg, "ALSA") {
				logger.Warning(logger.CategoryAudio, "ALSA error detected. Try the following:")
				logger.Info(logger.CategoryAudio, "1. Check audio hardware: 'aplay -l' and 'arecord -l'")
				logger.Info(logger.CategoryAudio, "2. Check for permission issues: 'sudo usermod -a -G audio $USER'")
				logger.Info(logger.CategoryAudio, "3. Create a minimal .asoundrc file in your home directory")
			}
		}
		r.mu.Unlock()
		return fmt.Errorf("failed to open audio stream: %w", err)
	}

	r.stream = stream

	// Starting the stream can block, so release lock
	r.mu.Unlock()
	err = r.stream.Start()
	r.mu.Lock()

	if err != nil {
		r.stream.Close()
		if r.config.Debug {
			logger.Error(logger.CategoryAudio, "Failed to start audio stream: %v", err)
		}
		r.mu.Unlock()
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	r.isRecording = true
	r.mu.Unlock()
	return nil
}

// Stop ends audio recording
func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isRecording {
		return nil
	}

	if r.stream != nil {
		err := r.stream.Stop()
		if err != nil {
			return fmt.Errorf("failed to stop audio stream: %w", err)
		}

		err = r.stream.Close()
		if err != nil {
			return fmt.Errorf("failed to close audio stream: %w", err)
		}
	}

	r.isRecording = false
	return nil
}

// Terminate should be called when the recorder is no longer needed
func (r *Recorder) Terminate() error {
	err := r.Stop()
	if err != nil {
		return err
	}

	if r.initialized {
		return portaudio.Terminate()
	}
	return nil
}

// Audio processing callback for PortAudio
func (r *Recorder) processAudio(in, _ []float32) {
	// Safety check for invalid or empty input data
	if len(in) == 0 {
		if r.config.Debug {
			logger.Warning(logger.CategoryAudio, "PortAudio callback received empty buffer")
		}
		return
	}

	// Lock to avoid race conditions with Stop/Terminate
	r.mu.Lock()
	defer r.mu.Unlock()

	// If no longer recording, don't process
	if !r.isRecording {
		return
	}

	// Check for corrupt audio samples (NaN or Inf)
	hasCorruptSamples := false
	for _, sample := range in {
		if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) {
			hasCorruptSamples = true
			break
		}
	}

	// Handle corrupt audio gracefully
	if hasCorruptSamples {
		if r.config.Debug {
			logger.Warning(logger.CategoryAudio, "Detected corrupt audio samples in PortAudio callback")
		}
		// Generate silence instead
		for i := range r.buffer {
			r.buffer[i] = 0
		}
	} else {
		// If data is valid, copy it to our buffer
		copy(r.buffer, in)
	}

	// If a callback is registered, send the data
	if r.dataCallback != nil {
		// Make a copy to avoid race conditions
		dataCopy := make([]float32, len(r.buffer))
		copy(dataCopy, r.buffer)

		// Execute callback outside the lock to prevent deadlocks
		// Since we're already copied the data, it's safe to unlock
		r.mu.Unlock()
		r.dataCallback(dataCopy)
		r.mu.Lock() // Reacquire the lock to match our defer
	}
}

// CalculateRMSLevel calculates the Root Mean Square level of audio data
func CalculateRMSLevel(buffer []float32) float32 {
	if len(buffer) == 0 {
		return 0
	}

	// Calculate sum of squares
	var sumOfSquares float64
	for _, sample := range buffer {
		sumOfSquares += float64(sample * sample)
	}

	// Calculate RMS
	meanSquare := sumOfSquares / float64(len(buffer))
	return float32(math.Sqrt(meanSquare))
}
