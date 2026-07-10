Please use the template from @LLM.md to create a prompt for implementing 'Create logging package' as defined in @TRACKER.MD . The prompt should follow the Task Execution and Verification Guide from @TASK_EXECUTION.md , including:
A clear breakdown of sub-tasks based on the task's effort level (5 story points)
Specific verification steps for each change
Documentation update requirements
A completion checklist
I will use this prompt in a new chat to implement the changes systematically.

follow the guidelines bellow to implement the following task. Keep the @logging_package_prompt.md updated so we do not lose tracker of where we are and context:

# Working with Cursor and Documentation

## Documentation Overview

### INTERFACE.MD

- **Purpose**: Documents all service interfaces and interactions
- **Key Sections**:
  - Interface Standards (REST, gRPC, Message Queue)
  - Current Service Interactions
  - Cross-Service Models
  - Planned Interactions
- **Use When**: Implementing new interfaces, modifying existing ones, or checking service interactions

### MANAGER.md

- **Purpose**: Records technical decisions and implementation patterns
- **Key Sections**:
  - Architecture Decisions
  - Implementation Patterns
  - Cross-Service Decisions
  - Implementation Guidelines
- **Use When**: Making technical decisions, implementing new patterns, or checking existing decisions

### README.md

- **Purpose**: Provides service overview and implementation standards
- **Key Sections**:
  - Service Overview
  - Implementation Standards
  - Development Guidelines
  - Cross-Service Integration
- **Use When**: Setting up new services, checking standards, or understanding the overall architecture

### TRACKER.MD

- **Purpose**: Tracks tasks, progress, and dependencies
- **Key Sections**:
  - Standardization Tasks
  - Shared Component Tasks
  - Implementation Tasks
  - Documentation Tasks
  - Testing Tasks
- **Use When**: Starting new tasks, updating progress, or checking dependencies

## Best Practices for Working with Cursor

### 1. Context-First Requests

Always start your requests with context from our documentation files:

```markdown
"Please implement [task] as defined in [file] section [section]"
```

Example:

```
"Please implement the error handling package as defined in TRACKER.MD under Common Package Development"
```

### 2. Multi-Reference Requests

When implementing features, reference multiple files for complete context:

```markdown
"Please implement [feature] following:

- Architecture from README.md section [section]
- Patterns from MANAGER.md section [section]
- Interfaces from INTERFACE.MD section [section]"
```

Example:

```
"Please implement the profile service following:
- Clean architecture from README.md Implementation Standards
- Error handling from MANAGER.md Implementation Patterns
- REST API standards from INTERFACE.MD"
```

### 3. Update Requests

When asking for documentation updates, be specific about the changes:

```markdown
"Please update [file] to reflect:

- [change] in [section]
- [new information] in [section]"
```

Example:

```
"Please update TRACKER.MD to reflect:
- Completion of error handling package in Common Package Development
- New dependency on metrics package in Integration Tasks"
```

### 4. Verification Requests

Ask Cursor to verify implementations against documentation:

```markdown
"Please verify [implementation] against:

- Standards in [file]
- Patterns in [file]
- Requirements in [file]"
```

Example:

```
"Please verify the new profile service against:
- Clean architecture standards in README.md
- Error handling patterns in MANAGER.md
- Interface requirements in INTERFACE.MD"
```

## Quick Reference Guide for Updating Files

### TRACKER.MD Updates

1. **Status Updates**:

   ```markdown
   | Task   | Priority   | Effort   | Status       | Dependencies   |
   | ------ | ---------- | -------- | ------------ | -------------- |
   | [Task] | [Priority] | [Effort] | [New Status] | [Dependencies] |
   ```

2. **Add Notes**:

   ```markdown
   ## Notes

   - [Date] [Task]: [Update details]
   - [Date] [Task]: [Blocker details]
   ```

### INTERFACE.MD Updates

1. **Add New Interface**:

   ```markdown
   | Endpoint   | Method   | Purpose   | Authentication |
   | ---------- | -------- | --------- | -------------- |
   | [Endpoint] | [Method] | [Purpose] | [Auth]         |
   ```

2. **Update Standards**:
   ```markdown
   | Standard   | Description   | Implementation   |
   | ---------- | ------------- | ---------------- |
   | [Standard] | [Description] | [Implementation] |
   ```

### MANAGER.md Updates

1. **Add New Decision**:
   ```markdown
   | Decision        | [Decision Name]  |
   | --------------- | ---------------- |
   | Context         | [Context]        |
   | Options         | [Options]        |
   | Decision Matrix | [Matrix]         |
   | Decision        | [Final Decision] |
   | Rationale       | [Rationale]      |
   | Impact          | [Impact]         |
   ```

### README.md Updates

1. **Add New Section**:

   ```markdown
   ### [Section Name]

   [Content following existing format]
   ```

2. **Update Standards**:

   ```markdown
   #### [Standard Name]

   [Content following existing format]
   ```

## Tips for Effective Cursor Interaction

1. **Be Specific**

   - Reference exact sections
   - Use consistent terminology
   - Specify file paths

2. **Provide Context**

   - Mention related files
   - Reference existing patterns
   - Include relevant decisions

3. **Ask for Verification**

   - Request documentation checks
   - Ask for pattern compliance
   - Verify against standards

4. **Request Updates**

   - Ask for documentation updates
   - Request status changes
   - Get implementation notes

5. **Maintain Consistency**
   - Use standard formats
   - Follow naming conventions
   - Keep documentation in sync

## Common Request Templates

### Implementation Request

```
Please implement [feature] following:
1. Architecture from README.md [section]
2. Patterns from MANAGER.md [section]
3. Interfaces from INTERFACE.md [section]
4. Track progress in TRACKER.md [section]
```

### Documentation Update Request

```
Please update [file] to reflect:
1. [change] in [section]
2. [new information] in [section]
3. Update related sections in [other files]
```

### Verification Request

```
Please verify [implementation] against:
1. Standards in README.md [section]
2. Patterns in MANAGER.md [section]
3. Interfaces in INTERFACE.md [section]
4. Requirements in TRACKER.md [section]
```

### Task Update Request

```
Please update TRACKER.md to reflect:
1. [task] status change to [status]
2. Add note: [details]
3. Update dependencies if needed
```

## Using LLM.md for Prompt Generation

### Purpose

The LLM.md file serves as a template generator for creating structured prompts for Cursor. It helps ensure consistent, well-structured requests that leverage our documentation effectively.

### How to Use LLM.md

1. **Basic Usage**

   ```
   Please use the template from services/LLM.md to create a prompt for implementing "[TASK_NAME]" as defined in [FILE].md. I will use this prompt in a new chat.
   ```

2. **Example Request**

   ```
   Please use the template from services/LLM.md to create a prompt for implementing "Rename all service folders to follow pattern" as defined in TRACKER.MD. I will use this prompt in a new chat.
   ```

3. **Using the Generated Prompt**
   - Review the generated prompt
   - Copy it
   - Start a new chat
   - Paste the prompt with a simple introduction:
   ```
   Please help me implement this task. Here's my structured request:
   [paste the generated prompt]
   ```

### Tips for Effective Prompt Generation

1. **Be Specific**

   - Use exact task names
   - Reference specific files
   - Include section references

2. **Provide Context**

   - Mention related tasks
   - Include dependencies
   - Reference patterns

3. **Review Generated Prompt**

   - Check completeness
   - Verify references
   - Ensure clarity

4. **Maintain Documentation**
   - Update task status
   - Document changes
   - Track progress
