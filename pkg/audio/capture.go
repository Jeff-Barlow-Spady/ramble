// Package audio provides a simplified audio capture system
package audio

import (
	"fmt"
	"math"
	"sync"

	"github.com/gordonklaus/portaudio"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// Capture handles microphone recording with minimal overhead
type Capture struct {
	// Configuration
	sampleRate      float64
	channels        int
	framesPerBuffer int
	debug           bool

	// Runtime state
	stream      *portaudio.Stream
	isActive    bool
	onAudio     func([]float32)
	audioBuffer []float32

	// Thread safety
	mu sync.Mutex
}

// New creates a new audio capture instance
func New(sampleRate float64, debug bool) (*Capture, error) {
	// Use reasonable defaults
	if sampleRate <= 0 {
		sampleRate = 16000 // 16kHz is standard for speech recognition
	}

	// Initialize PortAudio
	err := portaudio.Initialize()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize audio: %w", err)
	}

	// Create the capture instance
	capture := &Capture{
		sampleRate:      sampleRate,
		channels:        1, // Mono for speech recognition
		framesPerBuffer: 1024,
		debug:           debug,
		isActive:        false,
		audioBuffer:     make([]float32, 1024), // Pre-allocate buffer
	}

	if debug {
		// Log audio system information
		logger.Info(logger.CategoryAudio, "Audio system initialized: %s", portaudio.VersionText())

		// List available devices
		devices, err := portaudio.Devices()
		if err == nil && len(devices) > 0 {
			logger.Info(logger.CategoryAudio, "Available audio devices:")
			for i, dev := range devices {
				logger.Info(logger.CategoryAudio, "[%d] %s (in: %v, out: %v)",
					i, dev.Name, dev.MaxInputChannels > 0, dev.MaxOutputChannels > 0)
			}
		}
	}

	return capture, nil
}

// Start begins audio capture, calling the provided callback with audio data
func (c *Capture) Start(callback func([]float32)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isActive {
		return fmt.Errorf("audio capture already active")
	}

	// Store the callback
	c.onAudio = callback

	// Open the default input stream
	stream, err := portaudio.OpenDefaultStream(
		c.channels,        // Input channels
		0,                 // No output channels
		c.sampleRate,      // Sample rate
		c.framesPerBuffer, // Frames per buffer
		c.processAudio,    // Callback
	)

	if err != nil {
		return fmt.Errorf("failed to open audio stream: %w", err)
	}

	// Start streaming
	err = stream.Start()
	if err != nil {
		stream.Close()
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	c.stream = stream
	c.isActive = true

	if c.debug {
		logger.Info(logger.CategoryAudio, "Audio capture started")
	}

	return nil
}

// Stop ends audio capture
func (c *Capture) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isActive || c.stream == nil {
		return nil
	}

	// Stop and close the stream
	err := c.stream.Stop()
	if err != nil {
		return fmt.Errorf("failed to stop audio stream: %w", err)
	}

	err = c.stream.Close()
	if err != nil {
		return fmt.Errorf("failed to close audio stream: %w", err)
	}

	c.stream = nil
	c.isActive = false

	if c.debug {
		logger.Info(logger.CategoryAudio, "Audio capture stopped")
	}

	return nil
}

// Close performs cleanup, releasing PortAudio resources
func (c *Capture) Close() error {
	c.Stop()
	return portaudio.Terminate()
}

// IsActive returns whether audio capture is currently active
func (c *Capture) IsActive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isActive
}

// Audio callback function
func (c *Capture) processAudio(input, _ []float32) {
	if c.onAudio == nil {
		return
	}

	// Create a copy of the input data
	audioData := make([]float32, len(input))
	copy(audioData, input)

	// Send the audio data to the callback
	c.onAudio(audioData)
}

// CalculateLevel computes the RMS audio level from a buffer
func CalculateLevel(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}

	var sumSquares float32
	for _, sample := range samples {
		sumSquares += sample * sample
	}

	return float32(math.Sqrt(float64(sumSquares / float32(len(samples)))))
}
