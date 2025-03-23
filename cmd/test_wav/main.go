package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription/embed"
)

func main() {
	// Parse command-line flags
	debugFlag := flag.Bool("debug", true, "Enable debug logging")
	wavPath := flag.String("wav", "test.wav", "Path to test WAV file")
	logPath := flag.String("log", "test_wav.log", "Path to log file")
	flag.Parse()

	// Initialize logger
	logger.Initialize()
	if *debugFlag {
		logger.SetLevel(logger.LevelDebug)
		logger.Info(logger.CategoryApp, "Debug mode enabled - verbose logging active")
	}

	// Set up file logging
	logFile, err := os.Create(*logPath)
	if err != nil {
		fmt.Printf("Failed to create log file: %v\n", err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
		fmt.Printf("Logging to file: %s\n", *logPath)
	}

	// Resolve path to WAV file
	absWavPath, err := filepath.Abs(*wavPath)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to resolve WAV file path: %v", err)
		os.Exit(1)
	}

	// Check if WAV file exists
	if _, err := os.Stat(absWavPath); os.IsNotExist(err) {
		logger.Error(logger.CategoryApp, "WAV file does not exist: %s", absWavPath)
		os.Exit(1)
	}

	fmt.Printf("Processing WAV file: %s\n", absWavPath)
	logger.Info(logger.CategoryApp, "Processing WAV file: %s", absWavPath)

	// Load WAV file using the audio package
	audioData, err := audio.LoadFromWav(absWavPath)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to load WAV file: %v", err)
		os.Exit(1)
	}

	logger.Info(logger.CategoryApp, "Loaded WAV file with %d samples", len(audioData))
	fmt.Printf("Loaded WAV file with %d samples\n", len(audioData))

	// Resample the audio to 16kHz if needed
	audioData = audio.ResampleTo16k(audioData, 44100)
	logger.Info(logger.CategoryApp, "Resampled audio to 16kHz: %d samples", len(audioData))
	fmt.Printf("Resampled audio to 16kHz: %d samples\n", len(audioData))

	// Calculate audio level using audio package
	level := audio.CalculateRMSLevel(audioData)
	logger.Info(logger.CategoryApp, "Audio level (RMS): %.6f", level)
	fmt.Printf("Audio level (RMS): %.6f\n", level)

	// Create direct Whisper call on the full file
	tempFile, err := os.CreateTemp("", "whisper_full_*.wav")
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to create temp file: %v", err)
		os.Exit(1)
	}

	tempFileName := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempFileName)

	// Save the full audio data to the temp WAV file
	if err := audio.SaveToWav(audioData, tempFileName); err != nil {
		logger.Error(logger.CategoryApp, "Failed to save audio to WAV file: %v", err)
		os.Exit(1)
	}

	// Get embedded whisper executable path
	execPath, err := embed.GetWhisperExecutable()
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to get embedded executable: %v", err)
		os.Exit(1)
	}

	// Get model path
	modelPath, err := embed.ExtractModel("tiny")
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to get model path: %v", err)
		os.Exit(1)
	}

	fmt.Printf("Using whisper executable: %s\n", execPath)
	fmt.Printf("Using model: %s\n", modelPath)

	// Direct command execution
	args := []string{
		"-m", modelPath,
		"-f", tempFileName,
		"-otxt",   // Output as text
		"-nt",     // No timestamps
		"-t", "4", // Use 4 threads for processing
		"-ml", "1", // Set language model to multilingual
		"-l", "en", // English language
	}

	cmd := exec.Command(execPath, args...)

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to create stdout pipe: %v", err)
		os.Exit(1)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to create stderr pipe: %v", err)
		os.Exit(1)
	}

	// Start the command
	logger.Info(logger.CategoryApp, "Starting direct Whisper transcription")
	fmt.Printf("\nStarting direct Whisper transcription...\n")
	startTime := time.Now()

	if err := cmd.Start(); err != nil {
		logger.Error(logger.CategoryApp, "Failed to start Whisper command: %v", err)
		os.Exit(1)
	}

	// Capture stderr for debugging
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logger.Debug(logger.CategoryTranscription, "Whisper stderr: %s", scanner.Text())
		}
	}()

	// Read transcription result
	var resultBuilder strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "whisper_") {
			resultBuilder.WriteString(line)
			resultBuilder.WriteString(" ")
		}
	}

	// Wait for completion
	err = cmd.Wait()
	duration := time.Since(startTime)

	// Check output file
	outputFile := tempFileName + ".txt"
	outputText := ""

	if fileData, err := os.ReadFile(outputFile); err == nil {
		outputText = string(fileData)
	}

	// Report results
	fmt.Printf("\nTranscription completed in %.2f seconds\n", duration.Seconds())

	if err != nil {
		logger.Error(logger.CategoryApp, "Whisper process failed: %v", err)
		fmt.Printf("Error: %v\n", err)
	}

	if resultBuilder.Len() > 0 {
		result := strings.TrimSpace(resultBuilder.String())
		logger.Info(logger.CategoryApp, "Transcription result: %s", result)
		fmt.Printf("\nResult from stdout: %s\n", result)
	}

	if outputText != "" {
		logger.Info(logger.CategoryApp, "Output file content: %s", outputText)
		fmt.Printf("\nResult from output file: %s\n", outputText)
	}

	if resultBuilder.Len() == 0 && outputText == "" {
		logger.Warning(logger.CategoryApp, "No transcription result produced")
		fmt.Printf("\nNo transcription result produced\n")
	}
}
