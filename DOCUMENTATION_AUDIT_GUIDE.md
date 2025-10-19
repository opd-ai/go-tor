# Documentation Audit and Cleanup Guide

This guide provides comprehensive instructions for conducting systematic audits and cleanup of repository documentation.

## Overview

The go-tor repository uses a structured approach to documentation management that:
- Preserves critical information
- Maintains compliance requirements
- Eliminates redundancy
- Improves repository organization

## Quick Start

### Running an Audit

```bash
# Execute the audit script
./scripts/audit-documentation.sh

# Review the generated reports
cat audit-reports/audit-summary-*.md
```

### Understanding the Process

The audit process follows 6 phases:

1. **Pre-Audit Setup**: Backup and policy definition
2. **Classification**: Automated scanning and categorization
3. **Stakeholder Review**: Review period for proposed changes
4. **Consolidation**: Merge duplicate/related documents
5. **Execution**: Move, archive, or delete as approved
6. **Validation**: Verify changes and update indexes

## Classification Categories

### KEEP - Active Documents
**Criteria**: Currently used, legally required, or operationally critical

**Examples**:
- Core project documentation (README, ARCHITECTURE)
- Current phase reports (Phase 8.x, Phase 7.3.x)
- Security audit reports (3-year retention)
- Compliance matrices
- Executive summaries
- Remediation tracking documents

**Action**: Remain in main repository

### ARCHIVE - Historical Documents
**Criteria**: Historical value but not actively referenced

**Examples**:
- Completed phase reports (Phase 2-6)
- Historical implementation summaries
- Past audit reports beyond 3 years
- Superseded executive summaries

**Action**: Move to `/archive/` directory structure

**Location**: Organized by phase or year

### CONSOLIDATE - Duplicate/Related Documents
**Criteria**: Multiple documents covering similar topics

**Examples**:
- Multiple control protocol documents
- Duplicate compliance matrices (CSV vs. Markdown)
- Similar phase completion reports

**Action**: Review, merge if appropriate, keep authoritative version

### DELETE - Obsolete Documents
**Criteria**: No legal/compliance/historical value

**Examples**:
- Draft versions superseded by finals
- Temporary working documents
- Exact duplicates with no unique content
- Outdated reports with no retention requirement

**Action**: Remove after backup and stakeholder approval

**Note**: Currently, no documents in this category - all have retention value

## Detailed Procedures

### Phase 1: Pre-Audit Setup

#### 1.1 Create Backup
```bash
# Record current repository state
git rev-parse HEAD > /tmp/backup_commit.txt
git log -1 --format="%H %ai" > /tmp/backup_info.txt

# Repository is already version controlled - no additional backup needed
# For extra safety, you can create a branch
git branch backup-$(date +%Y%m%d)
```

#### 1.2 Review Retention Policy
- Read [docs/RETENTION_POLICY.md](docs/RETENTION_POLICY.md)
- Verify criteria align with project needs
- Update if necessary with stakeholder approval

#### 1.3 Identify Stakeholders
**Key Stakeholders**:
- Project maintainers
- Documentation owners
- Security/compliance team (if applicable)
- Active contributors

### Phase 2: Classification & Assessment

#### 2.1 Run Audit Script
```bash
./scripts/audit-documentation.sh
```

**Output**:
- CSV report with detailed file metadata
- Markdown summary with classifications
- Statistics and recommendations

#### 2.2 Review Classifications
```bash
# View the summary report
cat audit-reports/audit-summary-*.md

# Review detailed CSV for specific files
cat audit-reports/audit-*.csv | grep "CONSOLIDATE"
```

#### 2.3 Manual Review of Edge Cases
Documents classified as "REVIEW" require manual assessment:

1. Examine document content
2. Determine current relevance
3. Check for legal/compliance requirements
4. Assign appropriate classification
5. Update audit script logic if pattern emerges

### Phase 3: Stakeholder Review

#### 3.1 Distribute Audit Report
```bash
# Copy report to accessible location
cp audit-reports/audit-summary-*.md /tmp/audit-for-review.md

# Share with stakeholders via your preferred method
# (email, issue tracker, pull request, etc.)
```

#### 3.2 Review Period
- **Duration**: 5 business days minimum
- **Focus**: Verify no critical documents misclassified
- **Process**: Address concerns and update classifications

#### 3.3 Document Approvals
Create an approval log:
```markdown
## Stakeholder Approvals

- [Date] [Stakeholder Name] - Reviewed and approved
- [Date] [Stakeholder Name] - Approved with modifications (see notes)
```

### Phase 4: Consolidation Actions

#### 4.1 Review CONSOLIDATE Items
For each document marked for consolidation:

1. **Compare content**:
   ```bash
   diff file1.md file2.md
   ```

2. **Determine authoritative version**:
   - Most recent update
   - Most comprehensive content
   - Best maintained

3. **Plan consolidation**:
   - Merge content if needed
   - Create cross-references
   - Document supersession

#### 4.2 Execute Consolidation
Example for control protocol documents:
```bash
# Option A: Keep most comprehensive, archive others
mv IMPLEMENTATION_SUMMARY_CONTROL_PROTOCOL.md archive/superseded/

# Add cross-reference in remaining doc
echo "\n**Historical Versions**: See /archive/superseded/" >> CONTROL_PROTOCOL.md

# Option B: Merge and create authoritative version
# (Manual content merge required)
```

### Phase 5: Execution

#### 5.1 Archive Historical Documents
```bash
# Move Phase 2-6 completion reports
mv PHASE2_COMPLETION_REPORT.md archive/by-phase/phase2/
mv PHASE3_COMPLETION_REPORT.md archive/by-phase/phase3/
# ... etc

# Move implementation summaries
mv IMPLEMENTATION_SUMMARY_PHASE6.md archive/by-phase/phase6/
# ... etc

# Update git
git add archive/
git commit -m "Archive historical phase documentation"
```

#### 5.2 Update Documentation Index
Create or update main documentation index:
```markdown
## Documentation Structure

### Active Documentation
- [README.md](README.md) - Project overview
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) - System architecture
- ... (list all KEEP documents)

### Archived Documentation
See [archive/README.md](archive/README.md) for historical documents.

### Audit Reports
See [audit-reports/](audit-reports/) for audit history.
```

#### 5.3 Commit Changes
```bash
git add .
git commit -m "docs: Complete documentation audit and archival

- Archived 7 historical phase reports
- Consolidated 3 control protocol documents
- Created retention policy and audit framework
- Established archive structure

See audit-reports/audit-summary-*.md for details"
```

### Phase 6: Validation

#### 6.1 Generate Post-Cleanup Report
```bash
# Run audit again to verify
./scripts/audit-documentation.sh

# Compare before/after
diff audit-reports/audit-summary-[OLD].md audit-reports/audit-summary-[NEW].md
```

#### 6.2 Verify Changes
- Check that archived documents are accessible
- Verify no broken links
- Test cross-references
- Confirm backup is complete

#### 6.3 Update Maintenance Schedule
Add to project calendar:
- **Quarterly**: Quick scan for new archival candidates
- **Semi-annually**: Review archived documents for expiration
- **Annually**: Comprehensive audit like this one

## Automation and Tools

### Audit Script
**Location**: `scripts/audit-documentation.sh`

**Features**:
- Scans documentation files
- Applies classification rules
- Generates CSV and markdown reports
- Provides statistics and recommendations

**Customization**:
Edit the `classify_document()` function to adjust classification logic.

### Future Enhancements
Potential improvements:
- Add file size calculations (currently 0 due to timing)
- Implement automated consolidation detection
- Add link checking
- Create archive automation
- Integration with CI/CD

## Best Practices

### DO
✅ Always create backups before major changes  
✅ Obtain stakeholder approval before deletions  
✅ Document all decisions and rationale  
✅ Maintain audit trail in Git history  
✅ Schedule regular audits  
✅ Keep retention policy updated  
✅ Test restores from archives periodically  

### DON'T
❌ Delete documents without review period  
❌ Remove legally required documents  
❌ Archive current/active documentation  
❌ Skip backup verification  
❌ Make changes without stakeholder input  
❌ Ignore documents requiring manual review  

## Troubleshooting

### Audit Script Issues
**Problem**: Script fails with "stat: illegal option"  
**Solution**: Script tries both GNU and BSD stat formats

**Problem**: Classification seems incorrect  
**Solution**: Review and update `classify_document()` function logic

### Archive Access Issues
**Problem**: Can't find archived document  
**Solution**: Check Git history: `git log -- path/to/file.md`

### Restoration Needs
**Problem**: Need to restore archived document  
**Solution**: Copy from archive, update date, move to active location

## Examples

### Example 1: Quarterly Audit
```bash
# Run quick audit
./scripts/audit-documentation.sh

# Review summary
cat audit-reports/audit-summary-*.md

# If < 5 documents need action: handle immediately
# If >= 5 documents: schedule full review cycle
```

### Example 2: Phase Completion
```bash
# When Phase 9 completes:
# 1. Ensure Phase 9 documents are finalized
# 2. Run audit
# 3. Move Phase 7.x documents to archive
# 4. Update retention policy if needed
```

### Example 3: Compliance Audit
```bash
# For external audit:
# 1. Generate current audit report
./scripts/audit-documentation.sh

# 2. Verify security docs are retained
grep "SECURITY_AUDIT" audit-reports/audit-*.csv

# 3. Show retention policy
cat docs/RETENTION_POLICY.md

# 4. Demonstrate backup/restore capability
```

## Compliance and Legal Considerations

### Retention Requirements
- **Security audit reports**: 3 years
- **Compliance matrices**: 3 years
- **Phase completion reports**: 3 years
- **Implementation summaries**: 3 years

### Legal Hold
If documents are under legal hold:
1. Mark clearly in audit report
2. Never delete regardless of retention period
3. Document hold reason and authority
4. Review hold status periodically

### Privacy Considerations
Before deletion, verify documents don't contain:
- Personal information requiring specific retention
- Proprietary information with contractual obligations
- Evidence for potential legal matters

## Support and Resources

### Documentation
- [Retention Policy](docs/RETENTION_POLICY.md)
- [Archive README](archive/README.md)
- [Audit Reports](audit-reports/)

### Tools
- `scripts/audit-documentation.sh` - Main audit script
- Git history - Complete version control
- Archive directories - Organized storage

### Getting Help
- Review audit reports in `audit-reports/`
- Check Git history: `git log -- path/to/file`
- Contact project maintainers
- Consult retention policy documentation

---

**Version**: 1.0  
**Last Updated**: 2025-10-19  
**Maintained By**: Project Maintainers  
**Next Review**: 2026-04-19
