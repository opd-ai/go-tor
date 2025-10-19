# Documentation Archive

This directory contains archived documentation from the go-tor repository.

## Purpose

The archive preserves historical documentation that:
- Has completed its active lifecycle
- Maintains historical/compliance value
- Is no longer actively referenced in daily operations
- Must be retained per the [Retention Policy](../docs/RETENTION_POLICY.md)

## Archive Structure

### `/by-phase/`
Historical documentation organized by development phase:
- `/phase2/` - Phase 2 implementation artifacts
- `/phase3/` - Phase 3 implementation artifacts
- `/phase4/` - Phase 4 implementation artifacts
- `/phase5/` - Phase 5 implementation artifacts
- `/phase6/` - Phase 6 implementation artifacts
- `/phase7x/` - Phase 7.x sub-phases (7.1, 7.2)

### `/by-year/`
Time-based organization for cross-cutting documents:
- `/2024/` - Documents from 2024
- `/2025/` - Documents from 2025

### `/superseded/`
Documents that have been replaced by newer versions:
- Draft versions superseded by finals
- Previous revisions of ongoing documents
- Deprecated compliance matrices

### `/backups/`
Complete repository snapshots before major archival operations:
- `/YYYY-MM-DD/` - Dated backup snapshots
- Minimum 1-year retention
- Full file copies for rollback capability

## Access Guidelines

### Who Should Access This Archive?
- Project maintainers reviewing historical decisions
- Compliance officers conducting audits
- Developers researching implementation history
- New team members understanding project evolution

### When to Reference Archive?
- Investigating why certain design decisions were made
- Conducting compliance or security audits
- Researching similar past implementations
- Understanding deprecated features

## Archive Maintenance

### Retention Periods
- **Phase completion reports**: 3 years from phase completion
- **Implementation summaries**: 3 years from implementation
- **Security/compliance documents**: 3 years from creation
- **General documentation**: 1 year after archival

### Review Schedule
- **Annual Review**: Check for expired retention periods
- **Quarterly Scan**: Identify candidates for archival from main repository
- **As Needed**: Archive documents when phases complete or documents are superseded

## Accessing Archived Documents

All documents remain in Git history and can be accessed via:

1. **Direct file access**: Navigate to appropriate archive subdirectory
2. **Git history**: Use `git log -- <filepath>` to view document evolution
3. **Backup snapshots**: Restore complete state from `/backups/YYYY-MM-DD/`

## Restoration Process

To restore an archived document to active status:

1. Review document relevance with stakeholders
2. Copy from archive to appropriate active location
3. Update document metadata and "last reviewed" date
4. Remove from archive (or leave as historical copy)
5. Document restoration in audit log

## Contact

For questions about archived documents:
- Review the [Retention Policy](../docs/RETENTION_POLICY.md)
- Contact project maintainers
- Consult audit reports in `/audit-reports/`

---
**Last Updated**: 2025-10-19  
**Archive Version**: 1.0
