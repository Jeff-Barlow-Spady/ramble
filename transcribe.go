package transcribe

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	// Comment out whisper import to allow compilation without the library
	// Uncomment when the library is properly installed
	// "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"

	"encoding/binary"

	"github.com/jeff-barlow-spady/ramble/audio/processing"
	"github.com/jeff-barlow-spady/ramble/config"
)

// Using a blank interface to avoid direct dependency on whisper types
// This allows compiling without the whisper library
type whisperContext interface{}
type whisperParams interface{}

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

// Transcriber defines the interface for audio transcription
type Transcriber interface {
	// Start begins the transcription process with the given audio channel
	Start(audioChan <-chan []float32) (<-chan string, error)
	// Stop stops the transcription process
	Stop() error
}

// whisperTranscriber implements the Transcriber interface using whisper.cpp
type whisperTranscriber struct {
	isRunning bool
	stopChan  chan struct{}
}

// New creates a new Transcriber instance
func New(modelPath string) (Transcriber, error) {
	// No more mock transcriber - just one implementation
	return &whisperTranscriber{
		isRunning: false,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start begins the transcription process
func (t *whisperTranscriber) Start(audioChan <-chan []float32) (<-chan string, error) {
	if t.isRunning {
		return nil, errors.New("transcriber is already running")
	}

	t.isRunning = true
	t.stopChan = make(chan struct{})

	// Create a channel for text output
	textChan := make(chan string, 5)

	// Start transcription in a separate goroutine
	go func() {
		defer close(textChan)

		// Buffer for collecting audio data
		buffer := make([]float32, 0, 16000)

		for {
			select {
			case audioData, ok := <-audioChan:
				if !ok {
					return
				}

				// Add audio data to buffer
				buffer = append(buffer, audioData...)

				// When we have enough data, process it
				if len(buffer) >= 16000 {
					text, err := processing.TranscribeAudio(buffer)
					if err == nil && text != "" {
						textChan <- text
					}
					buffer = buffer[:0] // Clear buffer
				}

			case <-t.stopChan:
				return
			}
		}
	}()

	return textChan, nil
}

// Stop stops the transcription process
func (t *whisperTranscriber) Stop() error {
	if !t.isRunning {
		return nil
	}

	close(t.stopChan)
	t.isRunning = false
	return nil
}

// ExecutableTranscriber uses an external whisper executable
type executableTranscriber struct {
	executablePath string
	modelPath      string
	modelType      string
	isRunning      bool
	cmd            *exec.Cmd
	stopChan       chan struct{}
	tempWavFile    string
}

func newExecutableTranscriber(executablePath, modelPath, modelType string) (*executableTranscriber, error) {
	// Check if the executable exists and is executable
	if _, err := os.Stat(executablePath); err != nil {
		return nil, fmt.Errorf("executable not found: %w", err)
	}

	// Return the transcriber
	return &executableTranscriber{
		executablePath: executablePath,
		modelPath:      modelPath,
		modelType:      modelType,
		isRunning:      false,
		stopChan:       make(chan struct{}),
	}, nil
}

func (t *executableTranscriber) Start(audioChan <-chan []float32) (<-chan string, error) {
	if t.isRunning {
		return nil, errors.New("transcriber is already running")
	}

	log.Printf("Starting executable transcriber with executable: %s", t.executablePath)
	log.Printf("Using model path: %s and model type: %s", t.modelPath, t.modelType)

	// Create text channel for output
	textChan := make(chan string)
	t.stopChan = make(chan struct{})
	t.isRunning = true

	// First, try to use the .ramble audio_backups directory for temporary files
	var tempDir string
	audioBackupDir, err := config.GetAudioBackupDir()
	if err == nil {
		// Create a subdirectory for this session
		sessionDir := filepath.Join(audioBackupDir, fmt.Sprintf("session-%d", time.Now().UnixNano()))
		if err := os.MkdirAll(sessionDir, 0755); err == nil {
			log.Printf("Using .ramble audio_backups directory for temporary files: %s", sessionDir)
			tempDir = sessionDir
		} else {
			log.Printf("Failed to create session directory in .ramble: %v", err)
		}
	}

	// Fall back to system temp directory if .ramble directory couldn't be used
	if tempDir == "" {
		// Create a temporary directory for storing WAV files
		sysTemp, err := os.MkdirTemp("", "ramble-audio-*")
		if err != nil {
			log.Printf("ERROR: Failed to create temporary directory: %v", err)
			t.isRunning = false
			close(textChan)
			return nil, fmt.Errorf("failed to create temporary directory: %w", err)
		}
		tempDir = sysTemp
		log.Printf("Using system temp directory for temporary files: %s", tempDir)
	}

	// Store temp directory path for cleanup
	t.tempWavFile = filepath.Join(tempDir, "audio.wav")
	log.Printf("Will save audio to: %s", t.tempWavFile)

	// Start processing in a goroutine
	go func() {
		defer close(textChan)

		// Buffer to accumulate audio samples
		buffer := make([]float32, 0, 16000*30) // 30 seconds at 16kHz
		bufferTime := time.Now()

		// Process audio chunks
		for {
			select {
			case <-t.stopChan:
				log.Println("Executable transcription stopped")
				t.isRunning = false
				return

			case samples, ok := <-audioChan:
				if !ok {
					// Audio channel was closed, process any remaining audio
					if len(buffer) > 0 {
						log.Printf("Processing final audio buffer with %d samples", len(buffer))
						// Save buffer to WAV file
						if err := saveAudioToWav(buffer, t.tempWavFile); err != nil {
							log.Printf("Error saving WAV file: %v", err)
							return
						}

						// Process the WAV file with the executable
						text, err := t.processAudioWithExecutable(t.tempWavFile)
						if err != nil {
							log.Printf("Error processing audio with executable: %v", err)
							return
						}

						if text != "" {
							textChan <- text
						}
					}
					t.isRunning = false
					return
				}

				// Add the samples to our buffer
				buffer = append(buffer, samples...)

				// Process buffer if it's large enough or it's been long enough since last processing
				bufferSize := len(buffer)
				bufferDuration := float64(bufferSize) / 16000.0 // duration in seconds
				timeSinceLastProcess := time.Since(bufferTime)

				if bufferDuration >= 2.0 || timeSinceLastProcess > 5*time.Second {
					log.Printf("Processing audio buffer with %d samples (%.2f seconds)",
						bufferSize, bufferDuration)

					// Save buffer to WAV file
					if err := saveAudioToWav(buffer, t.tempWavFile); err != nil {
						log.Printf("Error saving WAV file: %v", err)
						continue
					}

					// Process the WAV file with the executable
					text, err := t.processAudioWithExecutable(t.tempWavFile)
					if err != nil {
						log.Printf("Error processing audio with executable: %v", err)
						// Continue even if there's an error, don't return
					} else if text != "" {
						textChan <- text
					}

					// Reset buffer
					buffer = buffer[:0]
					bufferTime = time.Now()
				}
			}
		}
	}()

	return textChan, nil
}

func (t *executableTranscriber) Stop() error {
	if !t.isRunning {
		return errors.New("transcriber is not running")
	}

	log.Println("Stopping executable transcriber")
	t.isRunning = false
	close(t.stopChan)

	// Kill the command if it's running
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}

	// Remove temporary file if it exists
	if t.tempWavFile != "" {
		os.Remove(t.tempWavFile)
	}

	return nil
}

// processAudioWithExecutable processes audio with an external executable
func (t *executableTranscriber) processAudioWithExecutable(wavFile string) (string, error) {
	// Check if the WAV file exists
	if _, err := os.Stat(wavFile); os.IsNotExist(err) {
		log.Printf("ERROR: WAV file does not exist: %s", wavFile)
		return "", fmt.Errorf("wav file does not exist: %w", err)
	}
	log.Printf("WAV file exists at: %s", wavFile)

	// Get the absolute path of the WAV file
	absWavPath, err := filepath.Abs(wavFile)
	if err != nil {
		log.Printf("ERROR: Cannot get absolute path for WAV file: %v", err)
	} else {
		log.Printf("Absolute WAV file path: %s", absWavPath)
		wavFile = absWavPath
	}

	// Check if the model file exists
	modelFile := filepath.Join(t.modelPath, t.modelType+".bin")
	if _, err := os.Stat(modelFile); os.IsNotExist(err) {
		log.Printf("WARNING: Model file does not exist: %s", modelFile)
	} else {
		log.Printf("Model file exists at: %s", modelFile)

		// Get the absolute path of the model file
		absModelPath, err := filepath.Abs(modelFile)
		if err != nil {
			log.Printf("ERROR: Cannot get absolute path for model file: %v", err)
		} else {
			log.Printf("Absolute model file path: %s", absModelPath)
			modelFile = absModelPath
		}
	}

	// Determine if we're running a snap package
	isSnapExecutable := strings.Contains(t.executablePath, "/snap/")

	// Build the command with absolute paths
	execType := detectExecutableType(t.executablePath)
	var args []string

	// For snap packages, we need special handling
	if isSnapExecutable {
		log.Printf("Detected snap package, using direct file access")
		// For snap packages, we'll try passing the file directly without copying
		// Note: This will only work if the files are in snap-accessible locations
	}

	switch execType {
	case ExecutableTypeWhisperCpp:
		log.Printf("Using whisper-cpp style arguments")
		if isSnapExecutable {
			// If this is a snap, try both methods - first with direct file paths
			args = []string{
				wavFile, // Pass the input file as a positional argument instead
				"-m", modelFile,
				"-otxt", // Output to text
				"-np",   // No printing except results
			}
		} else {
			args = []string{
				"-f", wavFile,
				"-m", modelFile,
				"-otxt", // Output to text
				"-np",   // No printing except results
			}
		}
	case ExecutableTypeWhisperGael:
		log.Printf("Using whisper-gael style arguments")
		args = []string{
			"--input", wavFile,
			"--model", modelFile,
			"--output_txt",
		}
	default:
		log.Printf("Using default (whisper-cpp) style arguments")
		if isSnapExecutable {
			// If this is a snap, try both methods - first with direct file paths
			args = []string{
				wavFile, // Pass the input file as a positional argument instead
				"-m", modelFile,
				"-otxt", // Output to text
				"-np",   // No printing except results
			}
		} else {
			args = []string{
				"-f", wavFile,
				"-m", modelFile,
				"-otxt",
				"-np",
			}
		}
	}

	log.Printf("Executing command: %s %v", t.executablePath, args)

	// Create the command
	cmd := exec.Command(t.executablePath, args...)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("ERROR: Failed to create stdout pipe: %v", err)
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("ERROR: Failed to create stderr pipe: %v", err)
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		log.Printf("ERROR: Failed to start command: %v", err)
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
		log.Printf("Executable stderr: %s", stderrOutput)
	}

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		// If the command failed and we're using a snap package, try the alternative method
		if isSnapExecutable && strings.Contains(stderrOutput, "input file not found") {
			log.Printf("Trying alternative method for snap package...")
			// Create a temporary shell script to run the command
			return t.processWithSnapWrapper(wavFile, modelFile)
		}

		log.Printf("ERROR: Command failed: %v", err)
		return "", fmt.Errorf("error processing audio with executable: %w", err)
	}

	// Trim extra whitespace
	transcribedText = strings.TrimSpace(transcribedText)
	log.Printf("Transcribed text: %s", transcribedText)

	return transcribedText, nil
}

// processWithSnapWrapper tries an alternative approach for snap packages
// by creating a temporary script that uses snap run
func (t *executableTranscriber) processWithSnapWrapper(wavFile, modelFile string) (string, error) {
	// Extract the snap name from the executable path
	parts := strings.Split(t.executablePath, "/")
	var snapName string
	for i, part := range parts {
		if part == "snap" && i+1 < len(parts) {
			snapName = parts[i+1]
			break
		}
	}

	if snapName == "" {
		snapName = "whisper-cpp" // Fallback
	}

	log.Printf("Using snap wrapper with snap name: %s", snapName)

	// Create a temporary directory for the output
	tmpDir, err := os.MkdirTemp("", "ramble-whisper-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the output file path
	outputFile := filepath.Join(tmpDir, "output.txt")

	// Build the snap command to run
	snapCmd := []string{
		"snap", "run", snapName,
		wavFile,
		"-m", modelFile,
		"-otxt",
		"-np",
		"-of", outputFile, // Output to a specific file
	}

	cmd := exec.Command(snapCmd[0], snapCmd[1:]...)

	// Get output for logging
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Snap command failed: %v", err)
		log.Printf("Command output: %s", string(output))
		return "", fmt.Errorf("snap command failed: %w", err)
	}

	// Read the output file
	data, err := os.ReadFile(outputFile)
	if err != nil {
		log.Printf("Failed to read output file: %v", err)
		return "", fmt.Errorf("failed to read output file: %w", err)
	}

	text := strings.TrimSpace(string(data))
	log.Printf("Transcribed text from snap wrapper: %s", text)

	return text, nil
}

// saveAudioToWav saves audio samples to a WAV file
func saveAudioToWav(samples []float32, outputPath string) error {
	log.Printf("Saving audio to WAV file: %s", outputPath)

	// Ensure the output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create output directory: %v", err)
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	log.Printf("Ensured output directory exists: %s", outputDir)

	// Check if there's enough audio data
	if len(samples) < 1000 {
		log.Printf("WARNING: Very small audio sample size: %d samples", len(samples))
	}

	// Get absolute path for logging
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		log.Printf("WARNING: Could not get absolute path for output file: %v", err)
	} else {
		log.Printf("Absolute path for WAV file: %s", absPath)
	}

	// Create the output file
	file, err := os.Create(outputPath)
	if err != nil {
		log.Printf("ERROR: Failed to create WAV file: %v", err)
		return fmt.Errorf("failed to create WAV file: %w", err)
	}
	defer file.Close()

	// WAV header format (44 bytes):
	// Offset  Size  Name             Description
	// 0       4     ChunkID          Contains "RIFF"
	// 4       4     ChunkSize        Size of the rest of the chunk: 4 + (8 + SubChunk1Size) + (8 + SubChunk2Size)
	// 8       4     Format           Contains "WAVE"
	// 12      4     Subchunk1ID      Contains "fmt "
	// 16      4     Subchunk1Size    16 for PCM
	// 20      2     AudioFormat      PCM = 1
	// 22      2     NumChannels      Mono = 1, Stereo = 2
	// 24      4     SampleRate       8000, 44100, etc.
	// 28      4     ByteRate         SampleRate * NumChannels * BitsPerSample/8
	// 32      2     BlockAlign       NumChannels * BitsPerSample/8
	// 34      2     BitsPerSample    8 bits = 8, 16 bits = 16, etc.
	// 36      4     Subchunk2ID      Contains "data"
	// 40      4     Subchunk2Size    Number of bytes in the data: NumSamples * NumChannels * BitsPerSample/8
	// 44      *     Data             The actual sound data

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
		log.Printf("ERROR: Failed to write RIFF header: %v", err)
		return err
	}

	// ChunkSize
	if err := binary.Write(file, binary.LittleEndian, uint32(chunkSize)); err != nil {
		log.Printf("ERROR: Failed to write chunk size: %v", err)
		return err
	}

	// Format: "WAVE"
	if _, err := file.Write([]byte("WAVE")); err != nil {
		log.Printf("ERROR: Failed to write WAVE format: %v", err)
		return err
	}

	// Subchunk1ID: "fmt "
	if _, err := file.Write([]byte("fmt ")); err != nil {
		log.Printf("ERROR: Failed to write fmt chunk: %v", err)
		return err
	}

	// Subchunk1Size: 16 for PCM
	if err := binary.Write(file, binary.LittleEndian, uint32(16)); err != nil {
		log.Printf("ERROR: Failed to write format chunk size: %v", err)
		return err
	}

	// AudioFormat: 1 for PCM
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil {
		log.Printf("ERROR: Failed to write audio format: %v", err)
		return err
	}

	// NumChannels: 1 for mono
	if err := binary.Write(file, binary.LittleEndian, uint16(numChannels)); err != nil {
		log.Printf("ERROR: Failed to write number of channels: %v", err)
		return err
	}

	// SampleRate: e.g., 16000 Hz
	if err := binary.Write(file, binary.LittleEndian, uint32(sampleRate)); err != nil {
		log.Printf("ERROR: Failed to write sample rate: %v", err)
		return err
	}

	// ByteRate: SampleRate * NumChannels * BitsPerSample/8
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	if err := binary.Write(file, binary.LittleEndian, uint32(byteRate)); err != nil {
		log.Printf("ERROR: Failed to write byte rate: %v", err)
		return err
	}

	// BlockAlign: NumChannels * BitsPerSample/8
	blockAlign := numChannels * bitsPerSample / 8
	if err := binary.Write(file, binary.LittleEndian, uint16(blockAlign)); err != nil {
		log.Printf("ERROR: Failed to write block align: %v", err)
		return err
	}

	// BitsPerSample: e.g., 16 bits
	if err := binary.Write(file, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		log.Printf("ERROR: Failed to write bits per sample: %v", err)
		return err
	}

	// Subchunk2ID: "data"
	if _, err := file.Write([]byte("data")); err != nil {
		log.Printf("ERROR: Failed to write data chunk: %v", err)
		return err
	}

	// Subchunk2Size: NumSamples * NumChannels * BitsPerSample/8
	if err := binary.Write(file, binary.LittleEndian, uint32(subChunk2Size)); err != nil {
		log.Printf("ERROR: Failed to write data chunk size: %v", err)
		return err
	}

	// Write the actual audio data as 16-bit PCM
	for _, sample := range samples {
		// Convert float32 [-1.0, 1.0] to int16 [-32768, 32767]
		sampleInt16 := int16(sample * 32767.0)
		if err := binary.Write(file, binary.LittleEndian, sampleInt16); err != nil {
			log.Printf("ERROR: Failed to write audio sample: %v", err)
			return err
		}
	}

	// Verify the file was written correctly
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("WARNING: Could not get file stats after writing: %v", err)
	} else {
		log.Printf("Successfully wrote WAV file with %d samples (%d bytes) to %s",
			len(samples), fileInfo.Size(), outputPath)
	}

	return nil
}

// abs returns the absolute value of a float32
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
