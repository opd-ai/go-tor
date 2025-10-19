#!/bin/bash
# Documentation Audit Script
# Scans repository for documentation files and classifies them based on retention policy

set -e

# Configuration
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_DIR="${REPO_ROOT}/audit-reports"
TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
AUDIT_REPORT="${OUTPUT_DIR}/audit-${TIMESTAMP}.csv"
SUMMARY_REPORT="${OUTPUT_DIR}/audit-summary-${TIMESTAMP}.md"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create output directory
mkdir -p "${OUTPUT_DIR}"

echo "================================================================"
echo "Documentation Audit - $(date)"
echo "================================================================"
echo ""

# Initialize CSV report
echo "File Path,File Name,Size (bytes),Last Modified,Classification,Category,Rationale" > "${AUDIT_REPORT}"

# Function to classify documents
classify_document() {
    local filepath="$1"
    local filename=$(basename "$filepath")
    local classification=""
    local category=""
    local rationale=""
    
    # README and core docs - KEEP
    if [[ "$filename" == "README.md" ]] || \
       [[ "$filename" == "LICENSE" ]] || \
       [[ "$filename" == "ARCHITECTURE.md" ]]; then
        classification="KEEP"
        category="Core Documentation"
        rationale="Essential project documentation"
    
    # Security and compliance - KEEP (3 year retention)
    elif [[ "$filename" =~ SECURITY_AUDIT ]] || \
         [[ "$filename" =~ COMPLIANCE_MATRIX ]] || \
         [[ "$filename" =~ PROOF_OF_CONCEPT ]]; then
        classification="KEEP"
        category="Security/Compliance"
        rationale="Required for compliance (3 year retention)"
    
    # Current phase documentation - KEEP
    elif [[ "$filename" =~ PHASE8 ]] || \
         [[ "$filename" =~ PHASE73 ]] || \
         [[ "$filename" =~ IMPLEMENTATION_SUMMARY_PHASE8 ]] || \
         [[ "$filename" =~ IMPLEMENTATION_SUMMARY_PHASE73 ]]; then
        classification="KEEP"
        category="Current Phase"
        rationale="Active development documentation"
    
    # Executive summaries - evaluate
    elif [[ "$filename" =~ EXECUTIVE ]]; then
        classification="KEEP"
        category="Executive Summary"
        rationale="High-level overview documents"
    
    # Remediation docs - KEEP
    elif [[ "$filename" =~ REMEDIATION ]]; then
        classification="KEEP"
        category="Remediation"
        rationale="Security remediation tracking"
    
    # Older phase completion reports - ARCHIVE
    elif [[ "$filename" =~ PHASE[2-7]_COMPLETION_REPORT ]]; then
        classification="ARCHIVE"
        category="Historical Phase Reports"
        rationale="Completed phase - historical value"
    
    # Older implementation summaries - ARCHIVE
    elif [[ "$filename" =~ IMPLEMENTATION_SUMMARY_PHASE[2-7] ]] && \
         [[ ! "$filename" =~ PHASE73 ]] && \
         [[ ! "$filename" =~ PHASE734 ]]; then
        classification="ARCHIVE"
        category="Historical Implementation"
        rationale="Completed implementation - historical value"
    
    # Control protocol docs - evaluate for consolidation
    elif [[ "$filename" =~ CONTROL_PROTOCOL ]]; then
        classification="CONSOLIDATE"
        category="Control Protocol"
        rationale="Multiple control protocol documents - assess for consolidation"
    
    # Audit deliverables - KEEP
    elif [[ "$filename" =~ AUDIT_DELIVERABLES ]]; then
        classification="KEEP"
        category="Audit Documentation"
        rationale="Audit process documentation"
    
    # Development/production docs - KEEP
    elif [[ "$filename" =~ (DEVELOPMENT|PRODUCTION|PERFORMANCE|LOGGING) ]]; then
        classification="KEEP"
        category="Operations Documentation"
        rationale="Operational reference material"
    
    # CSV compliance matrix - CONSOLIDATE
    elif [[ "$filename" =~ COMPLIANCE_MATRIX.csv ]]; then
        classification="CONSOLIDATE"
        category="Compliance"
        rationale="Superseded by markdown version - consider archiving"
    
    # Default: review required
    else
        classification="REVIEW"
        category="Unclassified"
        rationale="Manual review required"
    fi
    
    echo "$classification|$category|$rationale"
}

# Function to process file
process_file() {
    local filepath="$1"
    local filename=$(basename "$filepath")
    local size=$(stat -f%z "$filepath" 2>/dev/null || stat -c%s "$filepath" 2>/dev/null || echo "0")
    local modified=$(stat -f%Sm -t "%Y-%m-%d %H:%M:%S" "$filepath" 2>/dev/null || stat -c%y "$filepath" 2>/dev/null | cut -d'.' -f1)
    
    # Get classification
    local result=$(classify_document "$filepath")
    IFS='|' read -r classification category rationale <<< "$result"
    
    # Write to CSV (escape commas in fields)
    echo "\"$filepath\",\"$filename\",\"$size\",\"$modified\",\"$classification\",\"$category\",\"$rationale\"" >> "${AUDIT_REPORT}"
}

# Scan documentation files
echo "Scanning documentation files..."
echo ""

# Root level markdown files
find "${REPO_ROOT}" -maxdepth 1 -type f \( -name "*.md" -o -name "*.csv" \) -not -path "*/.git/*" | while read -r file; do
    process_file "$file"
done

# docs directory
if [ -d "${REPO_ROOT}/docs" ]; then
    find "${REPO_ROOT}/docs" -type f \( -name "*.md" -o -name "*.csv" \) | while read -r file; do
        process_file "$file"
    done
fi

# Count classifications
echo "================================================================"
echo "Audit Results Summary"
echo "================================================================"
echo ""

total_files=$(tail -n +2 "${AUDIT_REPORT}" | wc -l | tr -d ' ')
keep_count=$(tail -n +2 "${AUDIT_REPORT}" | grep -c "\"KEEP\"" || echo "0")
archive_count=$(tail -n +2 "${AUDIT_REPORT}" | grep -c "\"ARCHIVE\"" || echo "0")
consolidate_count=$(tail -n +2 "${AUDIT_REPORT}" | grep -c "\"CONSOLIDATE\"" || echo "0")
review_count=$(tail -n +2 "${AUDIT_REPORT}" | grep -c "\"REVIEW\"" || echo "0")

total_size=$(awk -F',' 'NR>1 {sum+=$3} END {print sum}' "${AUDIT_REPORT}")
total_size_mb=$(awk "BEGIN {printf \"%.2f\", $total_size / 1024 / 1024}")

echo -e "${GREEN}Total Files Scanned:${NC} $total_files"
echo -e "${GREEN}Total Size:${NC} ${total_size_mb} MB"
echo ""
echo -e "${GREEN}KEEP:${NC} $keep_count files"
echo -e "${YELLOW}ARCHIVE:${NC} $archive_count files"
echo -e "${YELLOW}CONSOLIDATE:${NC} $consolidate_count files"
echo -e "${RED}REVIEW REQUIRED:${NC} $review_count files"
echo ""

# Generate markdown summary report
cat > "${SUMMARY_REPORT}" << EOF
# Repository Documentation Audit Report

**Date**: $(date +"%Y-%m-%d")  
**Auditor**: Automated Audit Script  
**Repository**: go-tor  
**Commit**: $(cd "${REPO_ROOT}" && git rev-parse --short HEAD)

## Summary Metrics

- **Files Reviewed**: $total_files
- **Total Size**: ${total_size_mb} MB
- **Files to Keep (Active)**: $keep_count
- **Files to Archive**: $archive_count
- **Files to Consolidate**: $consolidate_count
- **Files Requiring Manual Review**: $review_count

## Classification Breakdown

### KEEP (Active/Required) - $keep_count files
Documents that must remain in active repository:
EOF

# Add KEEP files
tail -n +2 "${AUDIT_REPORT}" | grep "\"KEEP\"" | while IFS=',' read -r path name size modified class category rationale; do
    name=$(echo "$name" | sed 's/"//g')
    category=$(echo "$category" | sed 's/"//g')
    echo "- **$name** - $category" >> "${SUMMARY_REPORT}"
done

cat >> "${SUMMARY_REPORT}" << EOF

### ARCHIVE (Inactive but Retain) - $archive_count files
Documents with historical value:
EOF

# Add ARCHIVE files
tail -n +2 "${AUDIT_REPORT}" | grep "\"ARCHIVE\"" | while IFS=',' read -r path name size modified class category rationale; do
    name=$(echo "$name" | sed 's/"//g')
    category=$(echo "$category" | sed 's/"//g')
    echo "- **$name** - $category" >> "${SUMMARY_REPORT}"
done

cat >> "${SUMMARY_REPORT}" << EOF

### CONSOLIDATE (Review for Merging) - $consolidate_count files
Documents that may need consolidation:
EOF

# Add CONSOLIDATE files
tail -n +2 "${AUDIT_REPORT}" | grep "\"CONSOLIDATE\"" | while IFS=',' read -r path name size modified class category rationale; do
    name=$(echo "$name" | sed 's/"//g')
    category=$(echo "$category" | sed 's/"//g')
    rationale=$(echo "$rationale" | sed 's/"//g')
    echo "- **$name** - $category - $rationale" >> "${SUMMARY_REPORT}"
done

if [ "$review_count" -gt 0 ]; then
    cat >> "${SUMMARY_REPORT}" << EOF

### REVIEW REQUIRED - $review_count files
Documents requiring manual classification:
EOF

    # Add REVIEW files
    tail -n +2 "${AUDIT_REPORT}" | grep "\"REVIEW\"" | while IFS=',' read -r path name size modified class category rationale; do
        name=$(echo "$name" | sed 's/"//g')
        echo "- **$name**" >> "${SUMMARY_REPORT}"
    done
fi

cat >> "${SUMMARY_REPORT}" << EOF

## Retention Criteria Applied

Based on the [Retention Policy](../docs/RETENTION_POLICY.md):

1. **Core Documentation**: README, LICENSE, Architecture - Indefinite retention
2. **Security/Compliance**: Security audits, compliance matrices - 3 year retention
3. **Current Phase Documentation**: Phase 8.x and Phase 7.3.x - Active retention
4. **Historical Phase Reports**: Phase 2-7 completion reports - Archive (3 year retention)
5. **Implementation Summaries**: Older phases - Archive (3 year retention)
6. **Executive Summaries**: Current summaries - Keep, older - Archive (1 year)

## Recommendations

### Immediate Actions
1. **Review CONSOLIDATE items**: Evaluate control protocol documentation for consolidation
2. **Create archive structure**: Set up /archive directory for historical documents
3. **Review CSV compliance matrix**: Determine if CSV version is still needed alongside markdown

### Archive Structure Recommendation
\`\`\`
/archive/
  /by-year/
    /2024/
    /2025/
  /by-phase/
    /phase2/
    /phase3/
    /phase4/
    /phase5/
    /phase6/
  /superseded/
  /backups/
    /2025-10-19/
\`\`\`

### Storage Optimization
- **Potential Archive**: ~$archive_count files
- **Review for Consolidation**: $consolidate_count files
- **Estimated Storage Improvement**: Review after consolidation

## Stakeholder Review Required

Before executing any moves or deletions:
1. ✅ Retention policy reviewed and approved
2. ⏳ Stakeholder notification (5-business-day review period)
3. ⏳ Legal/compliance sign-off (if applicable)
4. ⏳ Backup verification
5. ⏳ Final approval from project maintainers

## Risk Mitigation Checklist

- [x] Full audit completed
- [x] Retention policy documented
- [x] Classification criteria applied
- [ ] Stakeholder notifications sent
- [ ] Archive structure created
- [ ] Backup created and verified
- [ ] Rollback procedure documented
- [ ] Final approval obtained

## Next Steps

1. **Review this report** with project maintainers
2. **Obtain stakeholder approvals** for proposed changes
3. **Create archive structure** if approved
4. **Execute archival process** in phases:
   - Phase 1: Move ARCHIVE items to archive structure
   - Phase 2: Consolidate CONSOLIDATE items
   - Phase 3: Update documentation index
5. **Establish maintenance schedule** for ongoing audits

## Backup Information

- **Backup Location**: /archive/backups/$(date +%Y-%m-%d)/
- **Retention Period**: 1 year minimum
- **Backup Method**: Git commit hash + file snapshots
- **Repository State**: $(cd "${REPO_ROOT}" && git rev-parse HEAD)

---

**Detailed Report**: [audit-${TIMESTAMP}.csv](./audit-${TIMESTAMP}.csv)  
**Generated**: $(date)
EOF

echo "================================================================"
echo "Audit Complete"
echo "================================================================"
echo ""
echo -e "${GREEN}Reports generated:${NC}"
echo "  - CSV Report: ${AUDIT_REPORT}"
echo "  - Summary Report: ${SUMMARY_REPORT}"
echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "  1. Review the summary report"
echo "  2. Obtain stakeholder approvals"
echo "  3. Create archive structure"
echo "  4. Execute approved changes"
echo ""
