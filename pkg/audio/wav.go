// Package audio provides functionality for capturing and processing audio
package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jeff-barlow-spady/ramble/pkg/logger"
)

// SaveToWav saves audio samples to a WAV file
func SaveToWav(samples []float32, outputPath string) error {
	logger.Debug(logger.CategoryAudio, "Saving audio to WAV file: %s", outputPath)

	// Ensure the output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		logger.Error(logger.CategoryAudio, "Failed to create output directory: %v", err)
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Check if there's enough audio data
	if len(samples) < 1000 {
		logger.Warning(logger.CategoryAudio, "Very small audio sample size: %d samples", len(samples))
	}

	// Create a WAV encoder
	f, err := os.Create(outputPath)
	if err != nil {
		logger.Error(logger.CategoryAudio, "Failed to create WAV file: %v", err)
		return fmt.Errorf("failed to create WAV file: %v", err)
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

// LoadFromWav loads a WAV file and returns the audio data as float32 samples
func LoadFromWav(filePath string) ([]float32, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV file: %w", err)
	}
	defer file.Close()

	// Read the WAV header (44 bytes)
	header := make([]byte, 44)
	_, err = io.ReadFull(file, header)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAV header: %w", err)
	}

	// Verify that it's a WAV file
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAV file")
	}

	// Get number of channels
	numChannels := int(binary.LittleEndian.Uint16(header[22:24]))
	if numChannels != 1 {
		logger.Warning(logger.CategoryAudio, "WAV file has %d channels, expected mono (1 channel)", numChannels)
	}

	// Get sample rate
	sampleRate := binary.LittleEndian.Uint32(header[24:28])
	if sampleRate != 16000 {
		logger.Warning(logger.CategoryAudio, "WAV file has sample rate %d Hz, expected 16000 Hz", sampleRate)
	}

	// Get bits per sample
	bitsPerSample := binary.LittleEndian.Uint16(header[34:36])
	logger.Info(logger.CategoryAudio, "WAV file: %d channels, %d Hz, %d bits per sample",
		numChannels, sampleRate, bitsPerSample)

	// Calculate data size (total file size - header size)
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	dataSize := fileInfo.Size() - 44

	// Read the audio data
	numSamples := int(dataSize) / (numChannels * int(bitsPerSample) / 8)
	logger.Info(logger.CategoryAudio, "WAV file has %d samples (%.2f seconds)",
		numSamples, float64(numSamples)/float64(sampleRate))

	// Read the audio data based on bits per sample
	var samples []float32
	if bitsPerSample == 16 {
		// 16-bit PCM
		pcmData := make([]int16, numSamples*numChannels)
		err = binary.Read(file, binary.LittleEndian, pcmData)
		if err != nil {
			return nil, fmt.Errorf("failed to read PCM data: %w", err)
		}

		// Convert int16 to float32 (normalized to [-1.0, 1.0])
		samples = make([]float32, numSamples)
		for i := 0; i < numSamples; i++ {
			// If stereo, average the channels
			if numChannels == 2 {
				samples[i] = (float32(pcmData[i*2]) + float32(pcmData[i*2+1])) / (2.0 * 32768.0)
			} else {
				samples[i] = float32(pcmData[i]) / 32768.0
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported bits per sample: %d", bitsPerSample)
	}

	return samples, nil
}

// ConvertToPCM16 converts float32 audio samples to 16-bit PCM byte format
// This is used for streaming audio data to whisper.cpp via stdin pipe
func ConvertToPCM16(samples []float32) []byte {
	// Calculate required buffer size (2 bytes per sample for 16-bit PCM)
	bufferSize := len(samples) * 2
	buffer := make([]byte, bufferSize)

	// Convert each float32 sample to int16 and then to bytes
	for i, sample := range samples {
		// Convert float32 [-1.0, 1.0] to int16 [-32768, 32767]
		// Clamp to ensure values don't exceed the range
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}

		var sampleInt16 int16
		if sample >= 0 {
			sampleInt16 = int16(sample * 32767.0)
		} else {
			sampleInt16 = int16(sample * 32768.0)
		}

		// Write the int16 as two bytes in little-endian format
		// First byte is the lower 8 bits
		buffer[i*2] = byte(sampleInt16 & 0xFF)
		// Second byte is the upper 8 bits
		buffer[i*2+1] = byte(sampleInt16 >> 8)
	}

	return buffer
}

// ResampleTo16k resamples audio data to 16kHz, which is what Whisper expects
func ResampleTo16k(samples []float32, originalSampleRate int) []float32 {
	if originalSampleRate == 16000 {
		// Already at the right sample rate
		return samples
	}

	// Calculate the ratio between sample rates
	ratio := float64(16000) / float64(originalSampleRate)
	newLength := int(float64(len(samples)) * ratio)

	// Create new array for resampled data
	resampled := make([]float32, newLength)

	// Simple linear interpolation for resampling
	for i := 0; i < newLength; i++ {
		// Find the position in the original array
		pos := float64(i) / ratio

		// Get the two samples we're between
		index := int(pos)
		if index >= len(samples)-1 {
			// Handle edge case at the end
			resampled[i] = samples[len(samples)-1]
			continue
		}

		// Calculate the weight (0.0 to 1.0)
		weight := float32(pos - float64(index))

		// Linear interpolation between the two samples
		resampled[i] = (1.0-weight)*samples[index] + weight*samples[index+1]
	}

	logger.Info(logger.CategoryAudio, "Resampled audio from %d Hz to 16000 Hz (from %d to %d samples)",
		originalSampleRate, len(samples), len(resampled))

	return resampled
}

// ProcessDspFilters applies any DSP filters to the audio data
func ProcessDspFilters(samples []float32) []float32 {
	// Apply any filters here (noise reduction, normalization, etc.)
	// Currently just returns the input samples
	return samples
}

// AppendToWav appends audio samples to an existing WAV file.
// This is primarily used for streaming audio to whisper, where
// we need to update a file that the whisper process is monitoring.
func AppendToWav(samples []float32, wavPath string) error {
	logger.Debug(logger.CategoryAudio, "Appending audio to WAV file: %s", wavPath)

	// Check if the file exists
	_, err := os.Stat(wavPath)
	if err != nil {
		return fmt.Errorf("cannot append to WAV file, file does not exist: %w", err)
	}

	// Open the file for read/write
	file, err := os.OpenFile(wavPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open WAV file for appending: %w", err)
	}
	defer file.Close()

	// First, read the existing header to extract necessary information
	header := make([]byte, 44)
	_, err = io.ReadFull(file, header)
	if err != nil {
		return fmt.Errorf("failed to read WAV header: %w", err)
	}

	// Verify it's a valid WAV file
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return fmt.Errorf("not a valid WAV file")
	}

	// Extract current data size from header
	existingDataSize := binary.LittleEndian.Uint32(header[40:44])

	// Calculate the new data size
	newDataSize := existingDataSize + uint32(len(samples)*2) // 2 bytes per sample for 16-bit PCM

	// Calculate the new chunk size (file size - 8)
	newChunkSize := 36 + newDataSize

	// Update the sizes in the header
	binary.LittleEndian.PutUint32(header[4:8], newChunkSize)  // Update ChunkSize
	binary.LittleEndian.PutUint32(header[40:44], newDataSize) // Update Subchunk2Size

	// Go back to the beginning of the file to write the updated header
	_, err = file.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to beginning of file: %w", err)
	}

	// Write the updated header
	_, err = file.Write(header)
	if err != nil {
		return fmt.Errorf("failed to write updated header: %w", err)
	}

	// Seek to the end of the existing data
	_, err = file.Seek(int64(44+existingDataSize), 0)
	if err != nil {
		return fmt.Errorf("failed to seek to end of data: %w", err)
	}

	// Append the new audio data
	for _, sample := range samples {
		// Convert float32 [-1.0, 1.0] to int16 [-32768, 32767]
		sampleInt16 := int16(sample * 32767.0)
		if err := binary.Write(file, binary.LittleEndian, sampleInt16); err != nil {
			return fmt.Errorf("failed to write sample data: %w", err)
		}
	}

	logger.Debug(logger.CategoryAudio, "Successfully appended %d samples to WAV file", len(samples))
	return nil
}
