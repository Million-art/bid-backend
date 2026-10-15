.PHONY: build run tidy

# Compile the entire project
build:
	go build ./...

# Run the admin API
run:
	go run cmd/api/main.go

# Clean up dependencies
tidy:
	go mod tidy
