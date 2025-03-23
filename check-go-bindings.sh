#!/bin/bash
set -e

echo "==== Checking Go bindings for whisper.cpp ===="

# Check if whisper.cpp directory exists
if [ ! -d "whisper.cpp" ]; then
    echo "ERROR: whisper.cpp directory not found"
    echo "Run ./build-go-bindings.sh first"
    exit 1
fi

# Check if models exist
if [ ! -d "whisper.cpp/models" ] || [ -z "$(ls -A whisper.cpp/models)" ]; then
    echo "ERROR: No models found in whisper.cpp/models"
    echo "Run ./build-go-bindings.sh to download models"
    exit 1
fi

# Check if Go bindings are built
if [ ! -d "whisper.cpp/bindings/go/pkg/whisper" ]; then
    echo "ERROR: Go bindings not found"
    echo "Run ./build-go-bindings.sh to build the Go bindings"
    exit 1
fi

# Create a temporary Go file to test the bindings
TEMP_FILE=$(mktemp)
echo "package main

import (
	\"fmt\"
	\"os\"
	\"path/filepath\"

	\"github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper\"
)

func main() {
	// Get model path
	dir, _ := os.Getwd()
	modelPath := filepath.Join(dir, \"whisper.cpp\", \"models\", \"ggml-tiny.bin\")

	// Check if model exists
	if _, err := os.Stat(modelPath); err != nil {
		fmt.Printf(\"Model not found at %s\\n\", modelPath)
		os.Exit(1)
	}

	// Try to load the model
	model, err := whisper.New(modelPath)
	if err != nil {
		fmt.Printf(\"Failed to load model: %v\\n\", err)
		os.Exit(1)
	}
	defer model.Close()

	fmt.Println(\"Successfully loaded model - Go bindings are working!\")
}
" > $TEMP_FILE

# Try to build and run the test program
echo "Testing Go bindings with a simple program..."
cd whisper.cpp/bindings/go
go run $TEMP_FILE
if [ $? -eq 0 ]; then
    echo "==== Go bindings are working correctly! ===="
else
    echo "ERROR: Failed to run Go bindings test"
    echo "Try running ./build-go-bindings.sh again to fix any issues"
    exit 1
fi

# Clean up
rm $TEMP_FILE

echo "You can now build the application with make build"