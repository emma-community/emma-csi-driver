# Emma CSI Driver Tests

This directory contains tests for the Emma CSI Driver.

## Test Structure

- **Unit Tests**: Located in `pkg/*/` directories alongside the source code
- **Integration Tests**: Located in `test/integration/`
- **End-to-End Tests**: Located in `test/e2e/`

## Running Tests

### Unit Tests

Unit tests test individual components in isolation with mocked dependencies.

```bash
# Run all unit tests
go test ./pkg/...

# Run tests for a specific package
go test ./pkg/emma/...
go test ./pkg/driver/...

# Run with verbose output
go test -v ./pkg/...

# Run with coverage
go test -cover ./pkg/...
```

### Integration Tests

Integration tests require real Emma API credentials and test against the Emma API.

**Prerequisites:**
- Emma.ms account with API access
- Service application created with "Manage" access level

**Setup:**
```bash
export EMMA_CLIENT_ID="your-client-id"
export EMMA_CLIENT_SECRET="your-client-secret"
export EMMA_API_URL="https://api.emma.ms/external"  # Optional, defaults to production
```

**Run:**
```bash
# Run integration tests
go test -tags=integration ./test/integration/...

# Run with verbose output
go test -v -tags=integration ./test/integration/...
```

**Note:** Integration tests will create and delete real volumes in your Emma account. Ensure you have appropriate permissions and understand the costs involved.

### End-to-End Tests

End-to-end tests require a running Kubernetes cluster with the Emma CSI driver deployed.

**Prerequisites:**
- Kubernetes cluster (1.20+)
- Emma CSI driver deployed to the cluster
- kubectl configured with cluster access
- Emma API credentials configured in the cluster

**Setup:**
```bash
export E2E_TEST=1
export KUBECONFIG=~/.kube/config  # Optional, defaults to ~/.kube/config
```

**Run:**
```bash
# Run e2e tests
go test -tags=e2e ./test/e2e/...

# Run with verbose output
go test -v -tags=e2e ./test/e2e/...

# Run a specific test
go test -tags=e2e -run TestPVCProvisioning ./test/e2e/...
```

**Note:** E2E tests will create and delete real Kubernetes resources (PVCs, Pods) and Emma volumes. Ensure you have appropriate permissions and understand the costs involved.

## Test Coverage

To generate a coverage report:

```bash
# Generate coverage for all packages
go test -coverprofile=coverage.out ./pkg/...

# View coverage in browser
go tool cover -html=coverage.out

# View coverage summary
go tool cover -func=coverage.out
```

## Continuous Integration

The project includes GitHub Actions workflows for automated testing:

- **Unit Tests**: Run on every push and pull request
- **Integration Tests**: Run on schedule or manual trigger (requires secrets)
- **E2E Tests**: Run on schedule or manual trigger (requires cluster)

See `.github/workflows/` for workflow definitions.

## Writing Tests

### Unit Test Guidelines

- Test files should be named `*_test.go`
- Place test files in the same package as the code being tested
- Use table-driven tests for multiple test cases
- Mock external dependencies (Emma API, filesystem, etc.)
- Focus on testing business logic and error handling

Example:
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expected    string
        expectError bool
    }{
        {
            name:     "valid input",
            input:    "test",
            expected: "TEST",
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MyFunction(tt.input)
            if tt.expectError && err == nil {
                t.Error("expected error but got none")
            }
            if !tt.expectError && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
            if result != tt.expected {
                t.Errorf("expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

### Integration Test Guidelines

- Use build tag `// +build integration` at the top of the file
- Check for required environment variables and skip if not set
- Clean up resources after tests (use defer)
- Test real API interactions
- Handle timeouts appropriately

### E2E Test Guidelines

- Use build tag `// +build e2e` at the top of the file
- Check for E2E_TEST environment variable and skip if not set
- Use unique names for resources (include timestamp)
- Clean up resources after tests (use defer)
- Wait for resources to reach desired state with timeouts
- Test complete user workflows

## Troubleshooting

### Unit Tests Fail

- Ensure all dependencies are installed: `go mod download`
- Check for syntax errors: `go build ./...`
- Run with verbose output to see detailed errors: `go test -v`

### Integration Tests Fail

- Verify Emma API credentials are correct
- Check network connectivity to Emma API
- Ensure you have sufficient permissions in Emma account
- Check Emma API status page for outages

### E2E Tests Fail

- Verify Kubernetes cluster is accessible: `kubectl cluster-info`
- Check CSI driver is deployed: `kubectl get pods -n kube-system | grep emma`
- Verify StorageClass exists: `kubectl get storageclass emma-ssd`
- Check driver logs: `kubectl logs -n kube-system -l app=emma-csi-controller`
- Ensure Emma API credentials are configured in cluster secret

## Test Maintenance

- Keep tests up to date with code changes
- Add tests for new features
- Update tests when fixing bugs
- Remove or update tests for deprecated features
- Maintain test documentation
