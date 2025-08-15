.PHONY: test cover build clean

# Build the application
build:
	go build -o exifupdater

# Run all tests
test:
	go test -v ./...

# Run a specific test. Example: make test TEST=TestNewExifTool
TEST ?= .
test-specific:
	go test -v -run $(TEST)

# Run tests with coverage and open in browser
cover:
	go test -coverprofile=coverage.out && go tool cover -html=coverage.out

# Clean build artifacts
clean:
	rm -f exifupdater coverage.out

# Install dependencies
deps:
	go mod tidy

# Run the linter
lint:
	golangci-lint run

# Run the application with default settings
run:
	go run .

# Help target to show available commands
help:
	@echo "Available targets:"
	@echo "  build        - Build the application"
	@echo "  test         - Run all tests"
	@echo "  test-specific - Run a specific test (e.g., make test TEST=TestNewExifTool)"
	@echo "  cover        - Run tests with coverage report"
	@echo "  deps         - Install dependencies"
	@echo "  lint         - Run the linter"
	@echo "  run          - Run the application"
	@echo "  clean        - Remove build artifacts"
