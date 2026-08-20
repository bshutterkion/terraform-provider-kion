# Testing the Kion Terraform Provider

This document describes how to run tests for the Kion Terraform provider.

## Test Types

The provider includes two types of tests:

### Unit Tests

Unit tests test individual functions and helpers without making API calls. They're fast and don't require credentials.

**Run unit tests:**

```bash
make test
```

Or directly:

```bash
go test -v ./internal/...
```

### Acceptance Tests

Acceptance tests create real infrastructure using a live Kion instance. They validate the full lifecycle of resources (create, read, update, delete).

**⚠️ WARNING:** Acceptance tests create and destroy real infrastructure and will incur costs.

## Running Acceptance Tests

### Prerequisites

1. **Go 1.21+** installed
2. **Terraform CLI 0.12.26+** installed
3. **Access to a Kion instance** (preferably a dedicated test/dev environment)
4. **Valid Kion credentials**

### Environment Setup

Set the following environment variables:

```bash
# Required: Kion API endpoint
export KION_API_URL="https://your-kion-instance.com"

# Required: At least one authentication method
export KION_API_KEY="your-api-key"
# OR
export KION_AUTH_TOKEN="your-auth-token"

# Required: Enable acceptance tests
export TF_ACC=1
```

### Running Tests

**Run all acceptance tests:**

```bash
make testacc
```

**Run specific resource tests:**

```bash
TF_ACC=1 go test -v -run TestAccKionLabel ./internal/provider/
```

**Run with verbose output:**

```bash
TF_ACC=1 go test -v -timeout 120m ./internal/provider/
```

### Test Execution Details

- Tests run in parallel (default: 4 parallel tests)
- Default timeout: 120 minutes
- Tests use the `test-acc` prefix for all created resources
- Tests automatically clean up resources after execution

## Sweepers

Sweepers help clean up orphaned test resources that may remain after failed tests.

**Run sweepers:**

```bash
make sweep
```

Or directly:

```bash
TF_ACC=1 go test -v -sweep=all -timeout 30m ./internal/provider/
```

**⚠️ WARNING:** Sweepers will destroy infrastructure. Only use in development accounts.

### How Sweepers Work

Sweepers:

1. List all resources of a given type
2. Identify test resources (those with `test-acc` prefix)
3. Delete identified test resources
4. Report any errors encountered

## Writing Tests

### Test File Structure

Test files follow Go conventions:

```
internal/provider/
├── provider.go
├── provider_test.go              # Test setup and helpers
├── kion_sweeper_test.go          # Sweeper registration
├── label_resource.go
├── label_resource_test.go        # Label resource tests
└── ...
```

### Example Acceptance Test

```go
func TestAccKionLabel_basic(t *testing.T) {
    resourceName := "kion_label.test"

    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        CheckDestroy:             testAccCheckKionLabelDestroy,
        Steps: []resource.TestStep{
            // Create and Read
            {
                Config: testAccKionLabelConfig_basic("test-acc-label", "environment", "test"),
                Check: resource.ComposeAggregateTestCheckFunc(
                    testAccCheckKionLabelExists(resourceName),
                    resource.TestCheckResourceAttr(resourceName, "key", "test-acc-label"),
                    resource.TestCheckResourceAttr(resourceName, "value", "environment"),
                ),
            },
            // ImportState testing
            {
                ResourceName:      resourceName,
                ImportState:       true,
                ImportStateVerify: true,
            },
            // Update and Read
            {
                Config: testAccKionLabelConfig_basic("test-acc-label", "production", "blue"),
                Check: resource.ComposeAggregateTestCheckFunc(
                    testAccCheckKionLabelExists(resourceName),
                    resource.TestCheckResourceAttr(resourceName, "value", "production"),
                ),
            },
        },
    })
}
```

### Test Best Practices

1. **Prefix test resources** with `test-acc` for sweeper identification
2. **Use random names** to avoid conflicts: `acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)`
3. **Test the full lifecycle:** Create → Read → Update → Delete
4. **Test import functionality** for all resources
5. **Clean up in CheckDestroy** to verify resources are properly deleted
6. **Use aggregate checks** to run all checks even if one fails

## Common Issues

### Authentication Failures

**Error:** `KION_API_URL must be set for acceptance tests`

**Solution:** Ensure environment variables are set:

```bash
export KION_API_URL="https://your-instance.com"
export KION_API_KEY="your-key"
```

### Resource Already Exists

**Error:** `Label already exists`

**Solution:** Run sweepers to clean up orphaned resources:

```bash
make sweep
```

### Test Timeout

**Error:** `test timed out after 30m`

**Solution:** Increase timeout:

```bash
TF_ACC=1 go test -v -timeout 180m ./internal/provider/
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Acceptance Tests

on:
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 2 * * *' # Run nightly

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run Acceptance Tests
        env:
          TF_ACC: 1
          KION_API_URL: ${{ secrets.KION_API_URL }}
          KION_API_KEY: ${{ secrets.KION_API_KEY }}
        run: make testacc

      - name: Run Sweepers (on failure)
        if: failure()
        env:
          TF_ACC: 1
          KION_API_URL: ${{ secrets.KION_API_URL }}
          KION_API_KEY: ${{ secrets.KION_API_KEY }}
        run: make sweep
```

## Resources

- [Terraform Plugin Testing Documentation](https://developer.hashicorp.com/terraform/plugin/testing)
- [Plugin Framework Testing](https://developer.hashicorp.com/terraform/plugin/framework/acctests)
- [Testing Patterns](https://developer.hashicorp.com/terraform/plugin/testing/testing-patterns)
