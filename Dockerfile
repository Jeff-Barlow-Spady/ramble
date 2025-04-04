FROM golang:1.21-bullseye as builder

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    libasound2-dev \
    libgtk-3-dev \
    libomp-dev \
    && rm -rf /var/lib/apt/lists/*

# Set up working directory
WORKDIR /app

# Copy only necessary files first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application
COPY . .

# Set up vendor directory
RUN chmod +x ./scripts/setup-vendor.sh && ./scripts/setup-vendor.sh

# Build application
RUN chmod +x ./scripts/build-dist.sh && ./scripts/build-dist.sh linux

# Create a smaller runtime image
FROM debian:bullseye-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    libasound2 \
    libgtk-3-0 \
    libomp5 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set up working directory
WORKDIR /app

# Copy built application and required files from builder
COPY --from=builder /app/dist/linux /app
COPY --from=builder /app/dist/ramble-linux-amd64.tar.gz /app/

# Set permissions
RUN chmod +x /app/ramble.sh

# Define entrypoint
ENTRYPOINT ["/app/ramble.sh"]