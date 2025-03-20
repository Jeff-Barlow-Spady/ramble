#!/bin/bash
set -e

# Create directory structure for embedded assets
mkdir -p pkg/transcription/embed/binaries/linux-amd64
mkdir -p pkg/transcription/embed/binaries/darwin-amd64
mkdir -p pkg/transcription/embed/binaries/windows-amd64
mkdir -p pkg/transcription/embed/models

# Create placeholder files to ensure directories are tracked in git
touch pkg/transcription/embed/binaries/.keep
touch pkg/transcription/embed/models/.keep
touch pkg/transcription/embed/binaries/linux-amd64/.keep
touch pkg/transcription/embed/binaries/darwin-amd64/.keep
touch pkg/transcription/embed/binaries/windows-amd64/.keep

echo "Directory structure for embed created successfully."