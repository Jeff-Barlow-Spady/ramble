# Creating a Release for Ramble

This document outlines the steps to create a release for the Ramble application.

## Prerequisites

- You have push access to the GitHub repository
- You have the GitHub CLI (`gh`) installed, or you can use the GitHub web interface
- Go version 1.23 or later is installed
- The repository has access to Whisper.cpp version 1.7.5

## Creating a Test Release

To create a test release and verify that the CI/CD pipeline properly builds and packages the application:

1. **Ensure all tests pass locally**:
   ```bash
   # Set up the vendor directory
   ./scripts/setup-vendor.sh

   # Run the tests
   export LD_LIBRARY_PATH=$(pwd)/vendor/whisper/lib:$LD_LIBRARY_PATH
   export CGO_CFLAGS="-I$(pwd)/vendor/whisper/include"
   export CGO_LDFLAGS="-L$(pwd)/vendor/whisper/lib"
   go test -tags=whisper_go ./tests/...
   ```

2. **Create a test release using GitHub CLI**:
   ```bash
   # Create a test release with a pre-release tag
   gh release create v0.1.0-test \
     --title "Test Release v0.1.0-test" \
     --notes "This is a test release to verify the CI/CD pipeline." \
     --prerelease
   ```

3. **Monitor the GitHub Actions workflow**:
   - Go to the "Actions" tab in the GitHub repository
   - You should see a workflow run for the release event
   - Wait for the workflow to complete

4. **Verify the release artifacts**:
   - Go to the "Releases" section in the GitHub repository
   - Check that the release has the following artifacts:
     - ramble-linux-amd64.tar.gz
     - ramble-windows-amd64.zip
     - ramble-macos-amd64.tar.gz

## Creating an Official Release

For an official release:

1. **Bump version numbers**:
   - Update version information in any relevant files

2. **Create release notes**:
   - Document changes, new features, bug fixes, etc.

3. **Create a release using GitHub CLI**:
   ```bash
   # Replace X.Y.Z with the actual version number
   gh release create vX.Y.Z \
     --title "Release vX.Y.Z" \
     --notes "Release notes for version X.Y.Z"
   ```

4. **Monitor the GitHub Actions workflow**:
   - Same as for test releases

5. **Verify and announce the release**:
   - Download and test the release artifacts
   - Announce the new release through appropriate channels

## Troubleshooting

If the build fails during the release process:

1. Check the GitHub Actions logs for details on what went wrong
2. Verify that `setup-vendor.sh` properly copies all necessary header files
3. Make sure the whisper.cpp dependency is properly set up in the repository
4. Fix any issues and create a new release