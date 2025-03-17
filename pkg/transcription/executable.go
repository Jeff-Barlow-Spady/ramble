// Package transcription provides speech-to-text functionality
package transcription

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// ExecutableType represents the type of whisper executable
type ExecutableType int

const (
	// ExecutableTypeWhisperCpp represents the whisper.cpp executable style
	ExecutableTypeWhisperCpp ExecutableType = iota
	// ExecutableTypeWhisperGael represents the whisper-gael (Python) executable style
	ExecutableTypeWhisperGael
	// ExecutableTypeUnknown represents an unknown executable type
	ExecutableTypeUnknown
)

// detectExecutableType determines the type of whisper executable
func detectExecutableType(execPath string) ExecutableType {
	if strings.Contains(execPath, "whisper-cpp") || strings.Contains(execPath, "main") {
		return ExecutableTypeWhisperCpp
	}
	if strings.Contains(execPath, "whisper-gael") || strings.Contains(execPath, "whisper.py") {
		return ExecutableTypeWhisperGael
	}
	// Default to whisper.cpp style
	return ExecutableTypeWhisperCpp
}

// ExecutableTranscriber uses an external whisper executable for transcription
type ExecutableTranscriber struct {
	config         Config
	executablePath string
	modelPath      string
	isRunning      bool
	cmd            *exec.Cmd
	stopChan       chan struct{}
	tempWavFile    string
	audioBuffer    []float32
	lastProcessed  time.Time
	mu             sync.Mutex
}

// NewExecutableTranscriber creates a new transcriber that uses an external executable
func NewExecutableTranscriber(config Config) (*ExecutableTranscriber, error) {
	// Find the whisper executable
	execPath, err := ensureExecutablePath(config)
	if err != nil {
		return nil, fmt.Errorf("failed to find executable: %w", err)
	}

	// Ensure the model exists
	modelFile, err := ensureModel(config)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure model: %w", err)
	}

	transcriber := &ExecutableTranscriber{
		config:         config,
		executablePath: execPath,
		modelPath:      modelFile,
		isRunning:      false,
		stopChan:       make(chan struct{}),
		audioBuffer:    make([]float32, 0),
		lastProcessed:  time.Now(),
	}

	logger.Info(logger.CategoryTranscription, "Created ExecutableTranscriber with %s, model %s",
		getExecutableTypeName(detectExecutableType(execPath)), config.ModelSize)

	return transcriber, nil
}

// UpdateConfig updates the transcriber's configuration
func (t *ExecutableTranscriber) UpdateConfig(config Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.config = config
	// Update model path based on new configuration
	t.modelPath = getModelPath(config.ModelPath, config.ModelSize)

	return nil
}

// GetModelInfo returns information about the current model
func (t *ExecutableTranscriber) GetModelInfo() (ModelSize, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.config.ModelSize, t.modelPath
}

// Transcribe converts audio data to text
func (t *ExecutableTranscriber) Transcribe(audioData []float32) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.config.Debug {
		logger.Debug(logger.CategoryTranscription, "Transcribing %d audio samples with executable", len(audioData))
	}

	// Create a temporary directory in the user's home folder
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	tempDir := filepath.Join(homeDir, ".local", "share", "ramble", "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the WAV file path
	wavFile := filepath.Join(tempDir, "audio.wav")

	// Save the audio data to a WAV file
	if err := saveAudioToWav(audioData, wavFile); err != nil {
		return "", fmt.Errorf("failed to save audio to WAV file: %w", err)
	}

	// Process the WAV file with the executable
	return t.processAudioWithExecutable(wavFile)
}

// Close frees resources associated with the transcriber
func (t *ExecutableTranscriber) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	logger.Info(logger.CategoryTranscription, "Closing executable transcriber")

	// Stop processing if still running
	if t.isRunning {
		close(t.stopChan)
		t.isRunning = false
	}

	// Remove temporary file if it exists
	if t.tempWavFile != "" {
		os.Remove(t.tempWavFile)
	}

	return nil
}

// ProcessAudioChunk processes a chunk of audio data and returns any transcribed text
func (t *ExecutableTranscriber) ProcessAudioChunk(audioData []float32) (string, error) {
	t.mu.Lock()

	// Append new audio data to our buffer
	t.audioBuffer = append(t.audioBuffer, audioData...)

	// Process more frequently - every 500ms of audio or 500ms of time
	// This ensures a more responsive feeling while transcribing
	shouldProcess := len(t.audioBuffer) >= 16000/2 || // 500ms of audio
		time.Since(t.lastProcessed) > 500*time.Millisecond

	if !shouldProcess {
		t.mu.Unlock()
		return "", nil
	}

	// Take a copy of the buffer for processing
	bufferCopy := make([]float32, len(t.audioBuffer))
	copy(bufferCopy, t.audioBuffer)

	// Clear the buffer
	t.audioBuffer = t.audioBuffer[:0]
	t.lastProcessed = time.Now()

	t.mu.Unlock()

	// Process the audio data
	text, err := t.Transcribe(bufferCopy)
	if err != nil {
		return "", fmt.Errorf("error transcribing audio: %w", err)
	}

	return text, nil
}

// processAudioWithExecutable processes audio with an external executable
func (t *ExecutableTranscriber) processAudioWithExecutable(wavFile string) (string, error) {
	// Check if the WAV file exists
	if _, err := os.Stat(wavFile); os.IsNotExist(err) {
		logger.Error(logger.CategoryTranscription, "WAV file does not exist: %s", wavFile)
		return "", fmt.Errorf("wav file does not exist: %w", err)
	}

	// Get absolute paths for files
	absWavPath, _ := filepath.Abs(wavFile)

	// Get the model file path
	modelFile := t.modelPath

	// Build the command arguments based on executable type
	execType := detectExecutableType(t.executablePath)
	var args []string

	switch execType {
	case ExecutableTypeWhisperCpp:
		logger.Debug(logger.CategoryTranscription, "Using whisper-cpp style arguments")
		args = []string{
			"-f", absWavPath,
			"-m", modelFile,
			"-otxt", // Output to text
			"-np",   // No printing except results
		}
	case ExecutableTypeWhisperGael:
		logger.Debug(logger.CategoryTranscription, "Using whisper-gael style arguments")
		args = []string{
			"--input", absWavPath,
			"--model", modelFile,
			"--output_txt",
		}
	default:
		logger.Debug(logger.CategoryTranscription, "Using default (whisper-cpp) style arguments")
		args = []string{
			"-f", absWavPath,
			"-m", modelFile,
			"-otxt",
			"-np",
		}
	}

	logger.Debug(logger.CategoryTranscription, "Executing command: %s %v", t.executablePath, args)

	// Create the command
	cmd := exec.Command(t.executablePath, args...)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to create stdout pipe: %v", err)
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to create stderr pipe: %v", err)
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to start command: %v", err)
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Read the output
	var transcribedText string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		transcribedText += line + " "
	}

	// Read stderr for error logging
	stderrBytes, _ := io.ReadAll(stderr)
	stderrOutput := string(stderrBytes)
	if stderrOutput != "" {
		logger.Warning(logger.CategoryTranscription, "Executable stderr: %s", stderrOutput)
	}

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Command failed: %v", err)
		return "", fmt.Errorf("error processing audio with executable: %w", err)
	}

	// Trim extra whitespace
	transcribedText = strings.TrimSpace(transcribedText)
	logger.Debug(logger.CategoryTranscription, "Transcribed text: %s", transcribedText)

	return transcribedText, nil
}

// saveAudioToWav saves audio samples to a WAV file
func saveAudioToWav(samples []float32, outputPath string) error {
	logger.Debug(logger.CategoryTranscription, "Saving audio to WAV file: %s", outputPath)

	// Ensure the output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to create output directory: %v", err)
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Check if there's enough audio data
	if len(samples) < 1000 {
		logger.Warning(logger.CategoryTranscription, "Very small audio sample size: %d samples", len(samples))
	}

	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to create WAV file: %v", err)
		return fmt.Errorf("failed to create WAV file: %w", err)
	}
	defer file.Close()

	// Parameters for WAV file
	numChannels := 1    // Mono
	sampleRate := 16000 // 16kHz (standard for Whisper)
	bitsPerSample := 16 // 16-bit PCM

	// Calculate sizes
	subChunk2Size := len(samples) * 2 // 2 bytes per sample (16-bit PCM)
	chunkSize := 36 + subChunk2Size

	// Write header
	// ChunkID: "RIFF"
	if _, err := file.Write([]byte("RIFF")); err != nil {
		return err
	}

	// ChunkSize
	if err := binary.Write(file, binary.LittleEndian, uint32(chunkSize)); err != nil {
		return err
	}

	// Format: "WAVE"
	if _, err := file.Write([]byte("WAVE")); err != nil {
		return err
	}

	// Subchunk1ID: "fmt "
	if _, err := file.Write([]byte("fmt ")); err != nil {
		return err
	}

	// Subchunk1Size: 16 for PCM
	if err := binary.Write(file, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}

	// AudioFormat: 1 for PCM
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}

	// NumChannels: 1 for mono
	if err := binary.Write(file, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}

	// SampleRate: e.g., 16000 Hz
	if err := binary.Write(file, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}

	// ByteRate: SampleRate * NumChannels * BitsPerSample/8
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	if err := binary.Write(file, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}

	// BlockAlign: NumChannels * BitsPerSample/8
	blockAlign := numChannels * bitsPerSample / 8
	if err := binary.Write(file, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}

	// BitsPerSample: e.g., 16 bits
	if err := binary.Write(file, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}

	// Subchunk2ID: "data"
	if _, err := file.Write([]byte("data")); err != nil {
		return err
	}

	// Subchunk2Size: NumSamples * NumChannels * BitsPerSample/8
	if err := binary.Write(file, binary.LittleEndian, uint32(subChunk2Size)); err != nil {
		return err
	}

	// Write the actual audio data as 16-bit PCM
	for _, sample := range samples {
		// Convert float32 [-1.0, 1.0] to int16 [-32768, 32767]
		sampleInt16 := int16(sample * 32767.0)
		if err := binary.Write(file, binary.LittleEndian, sampleInt16); err != nil {
			return err
		}
	}

	return nil
}
