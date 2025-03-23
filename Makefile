.PHONY: all build build-bindings clean check-bindings test run

# Default goal
all: build

# Build the application
build: check-bindings
	go build -o ramble ./cmd/ramble

# Build with Go bindings (recommended for better performance)
build-bindings:
	./build-go-bindings.sh
	go build -tags=whisper_go -o ramble ./cmd/ramble

# Check if Go bindings are available
check-bindings:
	@if [ -d "whisper.cpp/bindings/go/pkg/whisper" ]; then \
		echo "Go bindings detected, building with bindings for better performance..."; \
		go build -tags=whisper_go -o ramble ./cmd/ramble; \
	else \
		echo "Go bindings not detected, building without bindings..."; \
		echo "For better performance, run 'make build-bindings' first."; \
		go build -o ramble ./cmd/ramble; \
	fi

# Run the application
run: build
	./ramble

# Run tests
test:
	go test -v ./...

# Run tests with Go bindings
test-bindings:
	go test -tags=whisper_go -v ./...

# Clean build artifacts
clean:
	rm -f ramble
	@if [ -d "whisper.cpp" ]; then \
		cd whisper.cpp && make clean; \
	fi