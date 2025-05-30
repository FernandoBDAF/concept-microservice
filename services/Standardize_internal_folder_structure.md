Please, execute the following task from the @TRACKER.MD following this prompt:

# Implementation Request

## Task Context

Task: Standardize internal folder structure
Priority: High
Effort: 8 story points
Status: In Progress
Dependencies: None

## Documentation References

1. README.md

   - Section: Implementation Standards
   - Purpose: Define folder structure standards
   - Impact: Guides implementation of new structure

2. CONTEXT.MD

   - Section: System Architecture
   - Purpose: Understand service relationships
   - Impact: Ensure changes maintain architecture

3. INTERFACE.MD

   - Section: Service Interactions
   - Purpose: Identify affected interfaces
   - Impact: Plan interface updates

4. MANAGER.md

   - Section: Implementation Patterns
   - Purpose: Follow established patterns
   - Impact: Maintain consistency

5. TRACKER.MD
   - Section: File Structure Standardization
   - Purpose: Track implementation progress
   - Impact: Monitor task completion

## Requirements

1. Create standardized internal folder structure for all services
2. Implement consistent naming conventions
3. Update all import paths to reflect new structure
4. Ensure backward compatibility
5. Update all documentation references

## Constraints

- Must maintain all existing functionality
- Must follow clean architecture principles
- Must preserve existing service boundaries
- Must maintain backward compatibility

## Expected Output

- Standardized folder structure across all services
- Updated import paths
- Updated documentation
- Updated configuration files
- Verification test results

## Documentation Updates Required

1. TRACKER.MD

   - Section: File Structure Standardization
   - Changes: Update task status and progress
   - Reason: Track implementation progress

2. README.md

   - Section: Implementation Standards
   - Changes: Document new folder structure
   - Reason: Maintain documentation accuracy

3. CONTEXT.MD

   - Section: System Architecture
   - Changes: Update service structure documentation
   - Reason: Reflect new organization

4. INTERFACE.MD

   - Section: Service Interactions
   - Changes: Update interface documentation
   - Reason: Reflect new import paths

5. All the documenation of each reestructure service (.md files)

## Implementation Plan

### Phase 1: Analysis and Planning (2 story points) ✅

1. ✅ Inventory current folder structures
2. ✅ Map dependencies between services
3. ✅ Create standardization plan
4. ✅ Document current import paths

### Phase 2: Implementation (4 story points) 🔄

1. 🔄 Create new folder structure

   - ✅ Documented standardized structure
   - ❌ Implement in auth-service
   - ❌ Implement in profile-service
   - ❌ Implement in storage-service
   - ❌ Implement in cache-service
   - ❌ Implement in queue-service
   - ❌ Implement in worker-service
   - ❌ Implement in monitoring-service

2. 🔄 Update import paths

   - ❌ Update auth-service imports
   - ❌ Update profile-service imports
   - ❌ Update storage-service imports
   - ❌ Update cache-service imports
   - ❌ Update queue-service imports
   - ❌ Update worker-service imports
   - ❌ Update monitoring-service imports

3. 🔄 Update configuration files

   - ❌ Update auth-service configs
   - ❌ Update profile-service configs
   - ❌ Update storage-service configs
   - ❌ Update cache-service configs
   - ❌ Update queue-service configs
   - ❌ Update worker-service configs
   - ❌ Update monitoring-service configs

4. ✅ Update documentation
   - ✅ Update auth-service docs
   - ✅ Update profile-service docs
   - ✅ Update storage-service docs
   - ✅ Update cache-service docs
   - ✅ Update queue-service docs
   - ✅ Update worker-service docs
   - ✅ Update monitoring-service docs

### Phase 3: Verification (2 story points) ❌

1. ❌ Verify all services build
2. ❌ Run integration tests
3. ❌ Update documentation
4. ❌ Final review

## Current Status

### Completed Items

- ✅ Documentation of standardized folder structure
- ✅ Documentation updates for all services
- ✅ Analysis of current folder structures
- ✅ Dependency mapping
- ✅ Standardization plan creation
- ✅ Import path documentation

### In Progress

- 🔄 Implementation of new folder structure in services
- 🔄 Import path updates
- 🔄 Configuration file updates

### Pending Items

- ❌ Service-by-service implementation
- ❌ Build verification
- ❌ Integration testing
- ❌ Final documentation review

## Next Steps

1. Start with auth-service:

   - Create new folder structure
   - Move existing files
   - Update import paths
   - Update configuration files
   - Verify build

2. Continue with remaining services in order:

   - profile-service
   - storage-service
   - cache-service
   - queue-service
   - worker-service
   - monitoring-service

3. Run integration tests
4. Final documentation review

## Blockers/Questions

- None currently identified

## Task Completion Checklist

### Implementation Verification

- [ ] All requirements implemented
- [ ] Code follows standards
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] No regression issues

### Documentation Updates

- [x] README.md updated
- [x] CONTEXT.MD updated
- [x] INTERFACE.MD updated
- [x] MANAGER.md updated
- [x] TRACKER.MD updated

### Quality Checks

- [ ] Code review completed
- [ ] Tests passing
- [ ] Linting passed
- [ ] Build successful
- [ ] No security issues

## Task Completion Report Template

```markdown
# Task Completion Report

## Implementation Summary

- [Brief summary of what was implemented]

## Changes Made

- [List of all changes]

## Documentation Updates

- [List of documentation updates]

## Verification Results

- [Test results]
- [Performance metrics]
- [Security checks]

## Future Considerations

- [Any future improvements]
- [Potential optimizations]
- [Maintenance notes]
```
