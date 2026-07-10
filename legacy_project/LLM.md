# Cursor Prompt Generator

This file serves as a template generator for creating structured prompts for Cursor. When you ask Cursor to "use the template from services/LLM.md to create a prompt", it will use this file to generate a well-structured prompt that includes all necessary context and requirements.

## How This File Works

1. When you request a prompt generation, Cursor will:

   - Read this template file
   - Identify the task type
   - Fill in the template with task-specific details
   - Include all required documentation references
   - Generate a complete prompt

2. The generated prompt will include:
   - Task context from TRACKER.MD
   - Architecture requirements from README.md
   - Implementation patterns from MANAGER.md
   - Interface requirements from INTERFACE.MD
   - System context from CONTEXT.MD

## Implementation Task Template

```markdown
# Implementation Request

## Task Context

[Task details from TRACKER.MD]

## Documentation References

1. README.md

   - Section: [RELEVANT_SECTION]
   - Purpose: [WHY_NEEDED]
   - Impact: [HOW_IMPACTS_IMPLEMENTATION]

2. CONTEXT.MD

   - Section: [RELEVANT_SECTION]
   - Purpose: [WHY_NEEDED]
   - Impact: [HOW_IMPACTS_IMPLEMENTATION]

3. INTERFACE.MD

   - Section: [RELEVANT_SECTION]
   - Purpose: [WHY_NEEDED]
   - Impact: [HOW_IMPACTS_IMPLEMENTATION]

4. MANAGER.md

   - Section: [RELEVANT_SECTION]
   - Purpose: [WHY_NEEDED]
   - Impact: [HOW_IMPACTS_IMPLEMENTATION]

5. TRACKER.MD
   - Section: [RELEVANT_SECTION]
   - Purpose: [WHY_NEEDED]
   - Impact: [HOW_IMPACTS_IMPLEMENTATION]

## Requirements

1. [REQUIREMENT_1]
2. [REQUIREMENT_2]
3. [REQUIREMENT_3]

## Constraints

- Must follow [CONSTRAINT_1]
- Must adhere to [CONSTRAINT_2]
- Must consider [CONSTRAINT_3]

## Expected Output

- [OUTPUT_1]
- [OUTPUT_2]
- [OUTPUT_3]

## Documentation Updates Required

1. [FILE_NAME]

   - Section: [SECTION_TO_UPDATE]
   - Changes: [WHAT_TO_UPDATE]
   - Reason: [WHY_UPDATE_NEEDED]

2. [FILE_NAME]
   - Section: [SECTION_TO_UPDATE]
   - Changes: [WHAT_TO_UPDATE]
   - Reason: [WHY_UPDATE_NEEDED]

## Verification Requirements

- [VERIFICATION_1]
- [VERIFICATION_2]
- [VERIFICATION_3]
```

## Example Implementation Prompt

For the task "Rename all service folders to follow pattern":

```markdown
# Implementation Request

## Task Context

Task: Rename all service folders to follow pattern
Priority: High
Effort: Medium
Status: Not Started
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

1. Rename all service folders to follow the pattern: [service-name]-service
2. Update all import paths to reflect new folder names
3. Update all documentation references to new folder names

## Constraints

- Must maintain all existing functionality
- Must update all related configuration files
- Must consider impact on CI/CD pipelines

## Expected Output

- Renamed service folders
- Updated import paths
- Updated documentation
- Updated configuration files

## Documentation Updates Required

1. TRACKER.MD

   - Section: File Structure Standardization
   - Changes: Update task status
   - Reason: Track implementation progress

2. README.md
   - Section: Implementation Standards
   - Changes: Document new structure
   - Reason: Maintain documentation accuracy

## Verification Requirements

- All services build successfully
- All tests pass
- All documentation is up to date
- CI/CD pipelines run successfully
```

## How to Use This Template

1. **Request Prompt Generation**

   ```
   Please use the template from services/LLM.md to create a prompt for implementing "[TASK_NAME]" as defined in TRACKER.MD. I will use this prompt in a new chat.
   ```

2. **Review Generated Prompt**

   - Check all sections are filled
   - Verify documentation references
   - Ensure requirements are clear

3. **Use in New Chat**
   ```
   Please help me implement this task. Here's my structured request:
   [paste the generated prompt]
   ```

## Documentation Coverage Checklist

Before using a generated prompt, verify it includes:

1. **Architecture Context**

   - [ ] README.md architecture patterns
   - [ ] CONTEXT.MD system design
   - [ ] MANAGER.md decisions

2. **Implementation Details**

   - [ ] README.md standards
   - [ ] MANAGER.md patterns
   - [ ] TRACKER.MD requirements

3. **Interface Requirements**

   - [ ] INTERFACE.MD standards
   - [ ] CONTEXT.MD interactions
   - [ ] MANAGER.md decisions

4. **Task Context**
   - [ ] TRACKER.MD task details
   - [ ] Related tasks
   - [ ] Dependencies
