package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeff-barlow-spady/ramble/pkg/audio"
	"github.com/jeff-barlow-spady/ramble/pkg/logger"
	"github.com/jeff-barlow-spady/ramble/pkg/transcription"
)

func main() {
	// Parse command-line flags
	debugFlag := flag.Bool("debug", true, "Enable debug logging")
	wavPathFlag := flag.String("wav", "", "Path to WAV file for testing (required)")
	modelFlag := flag.String("model", "tiny", "Model size to use (tiny, base, small, medium, large)")
	flag.Parse()

	// Check required flags
	if *wavPathFlag == "" {
		fmt.Println("Error: -wav flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger
	logger.Initialize()
	if *debugFlag {
		logger.SetLevel(logger.LevelDebug)
		logger.Info(logger.CategoryApp, "Debug mode enabled - verbose logging active")
	}

	// Verify WAV file path
	wavPath, err := filepath.Abs(*wavPathFlag)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to resolve WAV file path: %v", err)
		os.Exit(1)
	}

	if _, err := os.Stat(wavPath); os.IsNotExist(err) {
		logger.Error(logger.CategoryApp, "WAV file not found: %s", wavPath)
		os.Exit(1)
	}

	fmt.Printf("Testing transcription of WAV file: %s\n", wavPath)

	// Load audio data
	fmt.Println("Loading audio file...")
	audioData, err := audio.LoadFromWav(wavPath)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to load WAV file: %v", err)
		os.Exit(1)
	}

	// Create transcriber
	fmt.Println("Creating transcriber...")
	config := transcription.DefaultConfig()
	config.ModelSize = transcription.ModelSize(*modelFlag)
	config.Debug = *debugFlag

	transcriber, err := transcription.NewTranscriber(config)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to create transcriber: %v", err)
		os.Exit(1)
	}
	defer transcriber.Close()

	// Set up a callback for streaming results
	if configTranscriber, ok := transcriber.(transcription.ConfigurableTranscriber); ok {
		configTranscriber.SetStreamingCallback(func(text string) {
			fmt.Printf("Streaming result: %s\n", text)
		})
	}

	// Process the audio
	fmt.Println("Processing audio...")
	startTime := time.Now()

	result, err := transcriber.ProcessAudioChunk(audioData)
	if err != nil {
		logger.Error(logger.CategoryApp, "Failed to process audio: %v", err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)

	// Display results
	fmt.Printf("\nTranscription completed in %.2f seconds\n", elapsed.Seconds())
	fmt.Printf("Result: %s\n", result)
}
