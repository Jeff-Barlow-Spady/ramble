#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default values
DEFAULT_TAG="latest"
IMAGE_NAME="ramble"
PUSH_IMAGE=false
REGISTRY=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --tag|-t)
      TAG="$2"
      shift # past argument
      shift # past value
      ;;
    --push|-p)
      PUSH_IMAGE=true
      shift # past argument
      ;;
    --registry|-r)
      REGISTRY="$2"
      shift # past argument
      shift # past value
      ;;
    --help|-h)
      echo "Usage: $0 [OPTIONS]"
      echo "Options:"
      echo "  --tag, -t TAG       Image tag (default: latest)"
      echo "  --push, -p          Push image to registry"
      echo "  --registry, -r URL  Registry URL (default: ghcr.io/YOUR_GITHUB_USERNAME)"
      echo "  --help, -h          Show this help"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Set default tag if not provided
TAG=${TAG:-$DEFAULT_TAG}

# Build the Docker image
echo "Building Docker image: $IMAGE_NAME:$TAG"
docker build -t $IMAGE_NAME:$TAG "$PROJECT_ROOT"

# Push to registry if requested
if [ "$PUSH_IMAGE" = true ]; then
  if [ -z "$REGISTRY" ]; then
    echo "Error: Registry URL is required for pushing. Use --registry option."
    exit 1
  fi

  FULL_IMAGE_NAME="$REGISTRY/$IMAGE_NAME:$TAG"
  echo "Tagging image as: $FULL_IMAGE_NAME"
  docker tag $IMAGE_NAME:$TAG $FULL_IMAGE_NAME

  echo "Pushing image to registry: $FULL_IMAGE_NAME"
  docker push $FULL_IMAGE_NAME

  echo "Image pushed successfully!"
else
  echo "Image built successfully! To push to a registry, use the --push option."
fi