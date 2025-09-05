.PHONY: build clean run test help

# Build the binary
build:
	go build -o outagemock main.go

# Clean build artifacts
clean:
	rm -f outagemock
	rm -f outagemock_temp_file

# Run with default parameters
run: build
	./outagemock

# Run with custom parameters (example)
run-example: build
	./outagemock -cpu 75 -memory 200 -fsize 500 -fpath /tmp/test_file -duration 60s -rampup 30s

# Test the build
test: build
	@echo "Build successful"

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build the outagemock binary"
	@echo "  clean       - Remove build artifacts and temp files"
	@echo "  run         - Run with default parameters"
	@echo "  run-example - Run with example parameters"
	@echo "  test        - Test the build"
	@echo "  help        - Show this help message"
