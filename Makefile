.PHONY: all build controller node clean test fmt vet deps docker-build docker-push docker-build-controller docker-build-node

# Build variables
BINARY_DIR=bin
CONTROLLER_BINARY=$(BINARY_DIR)/emma-csi-controller
NODE_BINARY=$(BINARY_DIR)/emma-csi-node

# Docker variables
IMAGE_REGISTRY?=docker.io
IMAGE_NAMESPACE?=emma
CONTROLLER_IMAGE_NAME?=$(IMAGE_REGISTRY)/$(IMAGE_NAMESPACE)/emma-csi-controller
NODE_IMAGE_NAME?=$(IMAGE_REGISTRY)/$(IMAGE_NAMESPACE)/emma-csi-node
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
IMAGE_TAG?=$(VERSION)

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE) -w -s"

all: build

build: controller node

controller:
	@echo "Building controller binary..."
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(CONTROLLER_BINARY) ./cmd/controller

node:
	@echo "Building node binary..."
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(NODE_BINARY) ./cmd/node

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)

test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

vet:
	@echo "Running go vet..."
	$(GOVET) ./...

deps:
	@echo "Downloading dependencies..."
	$(GOCMD) mod download
	$(GOCMD) mod tidy

# Docker build targets
docker-build: docker-build-controller docker-build-node

docker-build-controller:
	@echo "Building controller Docker image..."
	docker build \
		--target controller \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(CONTROLLER_IMAGE_NAME):$(IMAGE_TAG) \
		-t $(CONTROLLER_IMAGE_NAME):latest \
		-f Dockerfile .
	@echo "Controller image built: $(CONTROLLER_IMAGE_NAME):$(IMAGE_TAG)"

docker-build-node:
	@echo "Building node Docker image..."
	docker build \
		--target node \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(NODE_IMAGE_NAME):$(IMAGE_TAG) \
		-t $(NODE_IMAGE_NAME):latest \
		-f Dockerfile .
	@echo "Node image built: $(NODE_IMAGE_NAME):$(IMAGE_TAG)"

# Docker push targets
docker-push: docker-push-controller docker-push-node

docker-push-controller:
	@echo "Pushing controller Docker image..."
	docker push $(CONTROLLER_IMAGE_NAME):$(IMAGE_TAG)
	docker push $(CONTROLLER_IMAGE_NAME):latest
	@echo "Controller image pushed: $(CONTROLLER_IMAGE_NAME):$(IMAGE_TAG)"

docker-push-node:
	@echo "Pushing node Docker image..."
	docker push $(NODE_IMAGE_NAME):$(IMAGE_TAG)
	docker push $(NODE_IMAGE_NAME):latest
	@echo "Node image pushed: $(NODE_IMAGE_NAME):$(IMAGE_TAG)"

# Build and push in one command
release: docker-build docker-push
	@echo "Release complete: $(VERSION)"
