#!/bin/bash
set -e

echo "Testing file copy..."
mkdir -p vendor/whisper/include
mkdir -p vendor/whisper/lib
echo "Created directories"
touch test.h
echo "Created test file"
cp -v test.h vendor/whisper/include/
echo "Done!"