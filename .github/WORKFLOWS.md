# GitHub Actions CI/CD Pipeline Documentation

This repository uses a multi-file GitHub Actions workflow setup for automated testing, building, and releasing.

## Workflow Files

### 1. test.yml - Continuous Integration Testing

**Triggers:**
- Pull requests to `main` branch
- Direct pushes to `main` branch

**What it does:**
- Runs tests on Go versions 1.21.x and 1.22.x
- Checks code formatting with `gofmt`
- Runs `go vet` for static analysis
- Executes all tests with race detection and coverage
- Uploads coverage reports as artifacts

**How to use:**
- Automatically runs when you create a pull request
- Automatically runs when you push to main
- Check the "Actions" tab in GitHub to see test results

### 2. build.yml - Cross-Platform Build Verification

**Triggers:**
- Pushes to `main` branch

**What it does:**
- Builds binaries for multiple platforms:
  - Linux (amd64, arm64)
  - macOS/Darwin (amd64, arm64)
  - Windows (amd64)
- Uploads build artifacts for verification
- Artifacts are retained for 7 days

**How to use:**
- Automatically runs after code is merged to main
- Download artifacts from the workflow run to test binaries
- Verify cross-platform compatibility

### 3. release.yml - Automated Releases

**Triggers:**
1. **Nightly builds:** Daily at 00:00 UTC (via cron schedule)
2. **Versioned releases:** When pushing tags matching `v*.*.*` pattern
3. **Manual trigger:** Via GitHub Actions UI

**What it does:**

#### For Nightly Builds:
- Deletes existing 'nightly' tag and release (if exists)
- Creates new 'nightly' tag pointing to latest main
- Generates release notes from recent commits
- Builds cross-platform binaries
- Creates pre-release with all artifacts
- Includes SHA256 checksums for each binary

#### For Versioned Releases:
- Creates stable release from the tag
- Generates release notes from commits since last tag
- Builds cross-platform binaries
- Creates production release with all artifacts
- Includes SHA256 checksums for each binary

## Usage Guide

### Running Tests Locally

Before pushing code, run tests locally:

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Format code
make fmt

# Run static analysis
make vet
```

### Creating a Versioned Release

To create a new versioned release (e.g., v1.0.0):

```bash
# 1. Make sure you're on main and up to date
git checkout main
git pull

# 2. Create and push a version tag
git tag v1.0.0
git push origin v1.0.0
```

The release workflow will automatically:
- Detect the version tag
- Build binaries for all platforms
- Generate release notes
- Create a GitHub release
- Upload all binaries and checksums

### Triggering a Nightly Build

Nightly builds run automatically every day at midnight UTC. You can also trigger one manually:

1. Go to the "Actions" tab in GitHub
2. Select "Release" workflow
3. Click "Run workflow"
4. Select the main branch
5. Click "Run workflow"

### Downloading Release Artifacts

#### From Workflow Runs:
1. Go to "Actions" tab
2. Click on the workflow run
3. Scroll to "Artifacts" section
4. Download the desired artifact

#### From Releases:
1. Go to "Releases" section
2. Find the release (versioned or nightly)
3. Download binaries from the "Assets" section
4. Verify checksums:
   ```bash
   sha256sum -c tor-client-*.sha256
   ```

### Verifying Workflows are Working

#### Check Test Workflow:
1. Create a test branch and make a small change
2. Open a pull request
3. Check the "Checks" section in the PR
4. Verify tests run successfully on both Go versions

#### Check Build Workflow:
1. Merge a PR to main
2. Go to "Actions" tab
3. Find the "Build" workflow run
4. Verify all 5 platform builds succeed
5. Download artifacts to verify binaries

#### Check Release Workflow:
1. For nightly: Wait for next scheduled run or trigger manually
2. For versioned: Create and push a test tag like `v0.0.1-test`
3. Go to "Actions" tab
4. Verify the workflow completes successfully
5. Check "Releases" section for the new release
6. Download and test the binaries

## Workflow Permissions

All workflows use appropriate minimal permissions:

- **test.yml**: `contents: read` - Read-only access
- **build.yml**: `contents: read` - Read-only access
- **release.yml**: `contents: write` - Write access for creating releases

## Common Issues and Solutions

### Nightly Build Fails on First Run

**Symptom:** Nightly build fails when trying to delete non-existent tag/release

**Solution:** This is expected on the first run. The workflow uses `continue-on-error: true` for the delete step, so it will proceed to create the release.

### Version Tag Not Triggering Release

**Symptom:** Pushed a tag but release workflow didn't run

**Solution:** Ensure the tag matches the pattern `v[0-9]+.[0-9]+.[0-9]+` (e.g., v1.0.0, v2.1.3). Tags like `v1.0` or `1.0.0` won't match.

### Build Fails on Specific Platform

**Symptom:** Build succeeds for most platforms but fails for one

**Solution:** Check the build logs for platform-specific issues. Common causes:
- Platform-specific code that doesn't compile
- Missing build constraints (`//go:build`)
- CGO dependencies on Unix-only systems

### Test Coverage Artifacts Not Uploading

**Symptom:** Coverage artifacts don't appear in workflow results

**Solution:** This is normal if tests fail. Coverage is only uploaded after successful test runs.

## Best Practices

1. **Always run tests locally** before pushing
2. **Use semantic versioning** for release tags (v1.0.0, v1.2.3, etc.)
3. **Test nightly builds** periodically to ensure they work
4. **Check workflow status** in PRs before merging
5. **Review release notes** before publishing releases
6. **Verify checksums** when downloading binaries
7. **Keep workflows updated** with latest action versions

## Maintenance

### Updating Go Versions

To update the Go versions used in testing:

Edit `.github/workflows/test.yml`:
```yaml
strategy:
  matrix:
    go-version: ['1.22.x', '1.23.x']  # Update versions here
```

### Adding New Platforms

To add new build targets:

Edit `.github/workflows/build.yml` and `.github/workflows/release.yml`:
```yaml
strategy:
  matrix:
    include:
      - goos: linux
        goarch: amd64
      - goos: freebsd  # Add new platform
        goarch: amd64
```

### Changing Nightly Schedule

To change when nightly builds run:

Edit `.github/workflows/release.yml`:
```yaml
on:
  schedule:
    - cron: '0 2 * * *'  # Run at 2 AM UTC instead of midnight
```

## Security Notes

- All workflows use `GITHUB_TOKEN` automatically provided by GitHub
- No secrets need to be manually configured
- Workflows have minimal required permissions
- Dependencies are cached but checksummed via `go.sum`
- All build artifacts include SHA256 checksums for verification
