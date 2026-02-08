.PHONY: proto generate build run test clean

# Generate Protobuf Go code
proto:
	@echo "Generating Protobuf code..."
	@mkdir -p proto
	@protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/*.proto || echo "Note: protoc not found. Install Protocol Buffers compiler."

# Generate code (simplified - creates Go structs manually if protoc unavailable)
generate:
	@echo "Generating code..."
	@go generate ./...

# Build the application
build:
	@echo "Building application..."
	@go build -o bin/askmypdf main.go

# Run the application
run:
	@echo "Running application..."
	@go run main.go

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf proto/*.pb.go

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

