.PHONY: build clean run test help

# Build the binary (with optimizations disabled for debugging)
build:
	go build -gcflags="all=-N -l" -ldflags="-s=false" -o outagemock .

# Build with optimizations enabled (release build)
build-release:
	go build -o outagemock .

# Clean build artifacts
clean:
	rm -f outagemock
	rm -f outagemock_temp_file

# Run with default parameters
run: build
	./outagemock

# Run with custom parameters (example)
run-example: build
	./outagemock -cpu 75 -memory 200 -fsize 500 -fpath /data/test_file -duration 60s -rampup 30s

# Test the build
test: build
	@echo "Build successful"

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the outagemock binary (debug mode, no optimizations)"
	@echo "  build-release - Build the outagemock binary (release mode, with optimizations)"
	@echo "  clean         - Remove build artifacts and temp files"
	@echo "  run           - Run with default parameters"
	@echo "  run-example   - Run with example parameters"
	@echo "  test          - Test the build"
	@echo "  help          - Show this help message"
