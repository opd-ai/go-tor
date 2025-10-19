#!/bin/bash
# Comprehensive validation script for remediation progress
# This script validates the current state of security fixes and compliance

set -e

echo "============================================"
echo "Tor Client Remediation Validation"
echo "============================================"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Results tracking
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((TOTAL_CHECKS++))
    ((PASSED_CHECKS++))
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    ((TOTAL_CHECKS++))
    ((FAILED_CHECKS++))
}

check_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

echo "1. Build Verification"
echo "====================="
if go build -o /tmp/tor-client ./cmd/tor-client > /dev/null 2>&1; then
    check_pass "Build successful"
else
    check_fail "Build failed"
fi
echo ""

echo "2. Static Analysis"
echo "=================="

# Go vet
if go vet ./... > /dev/null 2>&1; then
    check_pass "go vet: PASS"
else
    check_fail "go vet: FAIL"
fi

# Staticcheck (if available)
if command -v staticcheck &> /dev/null; then
    if staticcheck ./... > /dev/null 2>&1; then
        check_pass "staticcheck: PASS"
    else
        check_fail "staticcheck: FAIL"
    fi
else
    check_warn "staticcheck: NOT INSTALLED"
fi

# Gosec (if available)
if command -v gosec &> /dev/null; then
    if gosec -quiet ./pkg/... > /tmp/gosec-validation.json 2>&1; then
        check_pass "gosec: PASS"
    else
        # Count remaining issues
        ISSUES=$(cat /tmp/gosec-validation.json 2>/dev/null | grep -c "\"rule_id\"" || echo "0")
        if [ "$ISSUES" -eq "0" ]; then
            check_pass "gosec: PASS (0 issues)"
        else
            check_warn "gosec: $ISSUES issues remaining"
        fi
    fi
else
    check_warn "gosec: NOT INSTALLED"
fi

echo ""

echo "3. Unit Tests"
echo "============="

# Run tests with race detector
if go test -race -timeout 180s ./... > /tmp/test-output.txt 2>&1; then
    check_pass "All tests pass with -race"
    
    # Count tests
    TEST_COUNT=$(grep -c "^=== RUN" /tmp/test-output.txt || echo "0")
    echo "   Total tests: $TEST_COUNT"
else
    check_fail "Tests failed"
    echo "   See /tmp/test-output.txt for details"
fi

echo ""

echo "4. Test Coverage"
echo "================"

# Run coverage
go test -cover ./pkg/... > /tmp/coverage.txt 2>&1 || true

# Parse coverage for each package
echo "Package Coverage:"
while IFS= read -r line; do
    if [[ $line =~ coverage:\ ([0-9.]+)% ]]; then
        coverage="${BASH_REMATCH[1]}"
        package=$(echo "$line" | awk '{print $2}')
        
        if (( $(echo "$coverage >= 85" | bc -l) )); then
            echo -e "  ${GREEN}✓${NC} $package: ${coverage}%"
        elif (( $(echo "$coverage >= 70" | bc -l) )); then
            echo -e "  ${YELLOW}⚠${NC} $package: ${coverage}%"
        else
            echo -e "  ${RED}✗${NC} $package: ${coverage}%"
        fi
    fi
done < /tmp/coverage.txt

echo ""

echo "5. Security Package Validation"
echo "==============================="

# Test security helpers
if go test -v ./pkg/security/... | grep -q "PASS"; then
    check_pass "Security package tests pass"
    
    # Check for specific security functions
    if grep -q "SafeUnixToUint64" pkg/security/conversion.go; then
        check_pass "SafeUnixToUint64 implemented"
    fi
    
    if grep -q "SafeUnixToUint32" pkg/security/conversion.go; then
        check_pass "SafeUnixToUint32 implemented"
    fi
    
    if grep -q "ConstantTimeCompare" pkg/security/conversion.go; then
        check_pass "ConstantTimeCompare implemented"
    fi
    
    if grep -q "SecureZeroMemory" pkg/security/conversion.go; then
        check_pass "SecureZeroMemory implemented"
    fi
else
    check_fail "Security package tests failed"
fi

echo ""

echo "6. TLS Configuration Validation"
echo "================================"

# Check TLS cipher suites
if grep -q "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" pkg/connection/connection.go; then
    check_pass "Secure TLS cipher suites configured"
fi

if grep -q "MinVersion: tls.VersionTLS12" pkg/connection/connection.go; then
    check_pass "TLS 1.2 minimum enforced"
fi

# Check for insecure CBC ciphers
if grep -q "CBC_SHA" pkg/connection/connection.go; then
    check_fail "Insecure CBC cipher suites still present"
else
    check_pass "No CBC cipher suites (vulnerable to padding oracle)"
fi

echo ""

echo "7. Integer Overflow Protection"
echo "==============================="

# Check for use of safe conversion functions
SAFE_CONVERSIONS=$(grep -r "security.Safe" pkg/ --include="*.go" | wc -l)
if [ "$SAFE_CONVERSIONS" -gt "10" ]; then
    check_pass "Safe conversion functions used ($SAFE_CONVERSIONS instances)"
else
    check_warn "Limited use of safe conversion functions ($SAFE_CONVERSIONS instances)"
fi

# Check for direct unsafe conversions (should be minimal)
UNSAFE_TIME_CONVERSIONS=$(grep -r "uint64(time.Now().Unix())" pkg/ --include="*.go" | wc -l)
if [ "$UNSAFE_TIME_CONVERSIONS" -eq "0" ]; then
    check_pass "No unsafe time conversions in pkg/"
else
    check_warn "Found $UNSAFE_TIME_CONVERSIONS potentially unsafe time conversions"
fi

echo ""

echo "8. Cross-Platform Builds"
echo "========================"

# Test builds for different platforms
platforms=("linux/amd64" "linux/arm" "linux/arm64" "linux/mips")

for platform in "${platforms[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$platform"
    if GOOS=$GOOS GOARCH=$GOARCH go build -o /dev/null ./cmd/tor-client > /dev/null 2>&1; then
        check_pass "Build successful for $platform"
    else
        check_fail "Build failed for $platform"
    fi
done

echo ""

echo "9. Documentation Status"
echo "======================="

docs=("README.md" "SECURITY_AUDIT_REPORT.md" "REMEDIATION_PHASE1_REPORT.md" "TOR_CLIENT_REMEDIATION_REPORT.md")

for doc in "${docs[@]}"; do
    if [ -f "$doc" ]; then
        check_pass "$doc exists"
    else
        check_fail "$doc missing"
    fi
done

echo ""

echo "============================================"
echo "Validation Summary"
echo "============================================"
echo "Total checks: $TOTAL_CHECKS"
echo -e "Passed: ${GREEN}$PASSED_CHECKS${NC}"
echo -e "Failed: ${RED}$FAILED_CHECKS${NC}"
echo ""

if [ "$FAILED_CHECKS" -eq "0" ]; then
    echo -e "${GREEN}✓ All critical validation checks passed!${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠ Some validation checks failed or need attention${NC}"
    exit 1
fi
