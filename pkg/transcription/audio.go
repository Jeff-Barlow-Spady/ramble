// Package transcription provides speech-to-text functionality
package transcription

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// saveAudioToWav saves audio samples to a WAV file
func saveAudioToWav(samples []float32, outputPath string) error {
	logger.Debug(logger.CategoryTranscription, "Saving audio to WAV file: %s", outputPath)

	// Ensure the output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to create output directory: %v", err)
		return fmt.Errorf("%w: failed to create output directory: %v",
			ErrTranscriptionFailed, err)
	}

	// Check if there's enough audio data
	if len(samples) < 1000 {
		logger.Warning(logger.CategoryTranscription, "Very small audio sample size: %d samples", len(samples))
	}

	// Create a WAV encoder
	f, err := os.Create(outputPath)
	if err != nil {
		logger.Error(logger.CategoryTranscription, "Failed to create WAV file: %v", err)
		return fmt.Errorf("%w: failed to create WAV file: %v",
			ErrTranscriptionFailed, err)
	}
	defer f.Close()

	// Parameters for WAV file
	numChannels := 1    // Mono
	sampleRate := 16000 // 16kHz (standard for Whisper)
	bitsPerSample := 16 // 16-bit PCM

	// Calculate sizes
	subChunk2Size := len(samples) * 2 // 2 bytes per sample (16-bit PCM)
	chunkSize := 36 + subChunk2Size

	// Write header
	// ChunkID: "RIFF"
	if _, err := f.Write([]byte("RIFF")); err != nil {
		return err
	}

	// ChunkSize
	if err := binary.Write(f, binary.LittleEndian, uint32(chunkSize)); err != nil {
		return err
	}

	// Format: "WAVE"
	if _, err := f.Write([]byte("WAVE")); err != nil {
		return err
	}

	// Subchunk1ID: "fmt "
	if _, err := f.Write([]byte("fmt ")); err != nil {
		return err
	}

	// Subchunk1Size: 16 for PCM
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}

	// AudioFormat: 1 for PCM
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}

	// NumChannels: 1 for mono
	if err := binary.Write(f, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}

	// SampleRate: e.g., 16000 Hz
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}

	// ByteRate: SampleRate * NumChannels * BitsPerSample/8
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	if err := binary.Write(f, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}

	// BlockAlign: NumChannels * BitsPerSample/8
	blockAlign := numChannels * bitsPerSample / 8
	if err := binary.Write(f, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}

	// BitsPerSample: e.g., 16 bits
	if err := binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}

	// Subchunk2ID: "data"
	if _, err := f.Write([]byte("data")); err != nil {
		return err
	}

	// Subchunk2Size: NumSamples * NumChannels * BitsPerSample/8
	if err := binary.Write(f, binary.LittleEndian, uint32(subChunk2Size)); err != nil {
		return err
	}

	// Write the actual audio data as 16-bit PCM
	for _, sample := range samples {
		// Convert float32 [-1.0, 1.0] to int16 [-32768, 32767]
		sampleInt16 := int16(sample * 32767.0)
		if err := binary.Write(f, binary.LittleEndian, sampleInt16); err != nil {
			return err
		}
	}

	return nil
}

// processDspFilters applies any DSP filters to the audio data
func processDspFilters(samples []float32) []float32 {
	// Apply any filters here (noise reduction, normalization, etc.)
	// Currently just returns the input samples
	return samples
}
