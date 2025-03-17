// Package audio provides functionality for capturing audio input
package audio

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/gordonklaus/portaudio"
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
			log.Printf("PortAudio initialization error: %v", err)

			// Check for common ALSA errors
			errMsg := err.Error()
			if strings.Contains(errMsg, "ALSA") {
				log.Println("ALSA error detected. This is usually due to a configuration issue.")
				log.Println("- Check if ALSA is properly installed")
				log.Println("- Try running 'aplay -l' to list audio devices")
				log.Println("- You may need to configure ~/.asoundrc or /etc/asound.conf")
			}
		}
		return nil, fmt.Errorf("failed to initialize PortAudio: %w", err)
	}

	recorder.initialized = true

	// Print available devices if in debug mode
	if config.Debug {
		devices, err := portaudio.Devices()
		if err != nil {
			log.Printf("Error getting audio devices: %v", err)
		} else {
			log.Println("Available audio devices:")
			for i, dev := range devices {
				log.Printf("[%d] %s (in: %v, out: %v)", i, dev.Name, dev.MaxInputChannels > 0, dev.MaxOutputChannels > 0)
			}
		}

		// Print PortAudio version information
		versionText := portaudio.VersionText()
		log.Printf("PortAudio version: %s", versionText)
	}

	return recorder, nil
}

// Start begins audio recording
// The provided callback will be called with audio data
func (r *Recorder) Start(callback func([]float32)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRecording {
		return errors.New("recorder is already running")
	}

	r.dataCallback = callback

	// Get default host API for logging purposes only
	if r.config.Debug {
		defaultHostApi, err := portaudio.DefaultHostApi()
		if err == nil {
			log.Printf("Using audio API: %s", defaultHostApi.Name)
			if defaultHostApi.DefaultInputDevice != nil {
				log.Printf("Default input device: %s", defaultHostApi.DefaultInputDevice.Name)
			} else {
				log.Println("Warning: No default input device found")
			}
		} else {
			log.Printf("Warning: Could not get default host API: %v", err)
		}
	}

	// Try to open the default audio stream
	// Just use the default stream which is most compatible
	stream, err := portaudio.OpenDefaultStream(
		r.config.Channels, // Input channels
		0,                 // No output channels
		r.config.SampleRate,
		r.config.FramesPerBuffer,
		r.processAudio,
	)

	if err != nil {
		if r.config.Debug {
			log.Printf("Failed to open audio stream: %v", err)

			// Provide guidance for common error cases
			errMsg := err.Error()
			if strings.Contains(errMsg, "ALSA") {
				log.Println("ALSA error detected. Try the following:")
				log.Println("1. Check audio hardware: 'aplay -l' and 'arecord -l'")
				log.Println("2. Check for permission issues: 'sudo usermod -a -G audio $USER'")
				log.Println("3. Create a minimal .asoundrc file in your home directory")
			}
		}
		return fmt.Errorf("failed to open audio stream: %w", err)
	}

	r.stream = stream
	err = r.stream.Start()
	if err != nil {
		r.stream.Close()
		if r.config.Debug {
			log.Printf("Failed to start audio stream: %v", err)
		}
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	r.isRecording = true
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
	// Copy input data to our buffer
	copy(r.buffer, in)

	// If a callback is registered, send the data
	if r.dataCallback != nil {
		// Make a copy to avoid race conditions
		dataCopy := make([]float32, len(r.buffer))
		copy(dataCopy, r.buffer)
		r.dataCallback(dataCopy)
	}
}
