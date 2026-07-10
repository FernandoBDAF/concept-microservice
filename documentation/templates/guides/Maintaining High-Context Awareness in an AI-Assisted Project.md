# Maintaining High-Context Awareness in an AI-Assisted Project

## Overview

Developing a complex web/mobile app with multiple services requires careful documentation of context so that both developers and AI assistants (like Cursor) can stay informed. This guide provides a structured plan to create standardized context files at the project root and within each service, helping maintain persistent, navigable, and auto-updating technical and project context throughout development.

## Project Structure and Context Files

### Root-Level Context Files (Project-Wide)

| File Name      | Purpose and Contents                                                                                                                                           |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `README.md`    | **Project Overview**: High-level description of the project, its goals, and overall architecture. Includes purpose, key features, and component relationships. |
| `CONTEXT.md`   | **Detailed Project Context & Architecture**: Expanded context including system architecture overview, domain knowledge, and background information.            |
| `MANAGER.md`   | **Project Management & Decisions Log**: Running log of technical decisions, design choices, and trade-offs made across the project.                            |
| `TRACKER.md`   | **Tasks & Blockers Tracker**: Living to-do list and progress tracker for the project.                                                                          |
| `INTERFACE.md` | **Inter-Service Relationships**: Documentation of how all services interact, including API endpoints, message queues, and shared resources.                    |

### Service-Level Context Files

For each service (e.g., `api/`, `mobile/`, `worker/`), include:

| File Name                 | Purpose and Contents                                                                           |
| ------------------------- | ---------------------------------------------------------------------------------------------- |
| `README.md`               | **Service Overview**: Purpose, setup instructions, and role in the overall system.             |
| `CONTEXT.md`              | **Service Technical Context**: Technical details, architecture, and service-local decisions.   |
| `INTERFACE.md`            | **Service Interface Details**: How this service connects to others, including APIs and queues. |
| `TRACKER.md`              | **Service Task Tracker**: To-do list specific to this service.                                 |
| `DECISIONS.md` (Optional) | **Service Decision Log**: Service-specific design decisions and history.                       |

## Directory Layout Example

```
project-root/
├── README.md           # Project goals, overview, high-level architecture
├── CONTEXT.md          # Detailed context and architecture info
├── MANAGER.md          # Decisions and trade-offs log (project-wide)
├── TRACKER.md          # Global tasks and blockers
├── INTERFACE.md        # All inter-service relationships
├── api/                # API service directory
│   ├── README.md       # API service overview and usage
│   ├── CONTEXT.md      # API-specific technical context
│   ├── INTERFACE.md    # API interfaces
│   ├── TRACKER.md      # API-specific tasks & issues
│   └── ... (code)
├── mobile/             # Mobile app service directory
│   ├── README.md       # Mobile app overview
│   ├── CONTEXT.md      # Mobile-specific context
│   ├── INTERFACE.md    # Mobile interfaces
│   ├── TRACKER.md      # Mobile-specific tasks
│   └── ... (code)
└── worker/             # Worker service directory
    ├── README.md       # Worker service overview
    ├── CONTEXT.md      # Worker-specific context
    ├── INTERFACE.md    # Worker interfaces
    ├── TRACKER.md      # Worker-specific tasks
    └── ... (code)
```

## Capturing Key Project Information

### 1. Project Goals & High-Level Architecture

- Clearly state project purpose, target users, and primary features
- Summarize high-level architecture with diagrams or component lists
- Use `CONTEXT.md` or `ARCHITECTURE.md` for detailed architecture documentation

### 2. Technical Decisions and Trade-offs

- Record significant decisions in `MANAGER.md` or `DECISIONS.md`
- Include:
  - Date of decision
  - Options considered
  - Chosen solution
  - Rationale
  - Trade-offs

### 3. Inter-Service Relationships

- Document in `INTERFACE.md`:
  - API endpoints
  - Message queue topics
  - Shared resources
  - Data flow
  - Integration points

### 4. Tasks, Blockers, and Historical Notes

- Use `TRACKER.md` for:
  - To-do items
  - In-progress tasks
  - Completed work
  - Current blockers
  - Historical notes

## Keeping Context Files Updated

### Best Practices

1. **Documentation Routine**

   - Update context as part of coding
   - Document changes in the same commit
   - Make it a habit

2. **Development Process Integration**

   - Include documentation checks in PR templates
   - Review documentation in code reviews
   - Consider automated documentation checks

3. **Automation**

   - Generate documentation from code
   - Sync with project management tools
   - Automate repetitive documentation tasks

4. **Lightweight and Regular Updates**

   - Make small, frequent updates
   - Dedicate time for documentation
   - Avoid documentation debt

5. **Templates and Consistent Format**

   - Use standard formats
   - Create templates for common sections
   - Maintain consistency

6. **Regular Maintenance**
   - Prune outdated information
   - Archive completed features
   - Keep files focused and relevant

## Optimizing for AI Assistance

### Formatting Guidelines

- Use clear Markdown structure
- Include descriptive headings
- Use tables for structured data
- Keep information scannable

### File Organization

- Keep files focused and relevant
- Maintain reasonable file sizes
- Use clear naming conventions
- Link related documents

### Context Management

- Update context regularly
- Version control documentation
- Use automation where possible
- Maintain clear relationships

## Sources

1. [Codebase Context Specification](https://codebasecontext.org/)
2. [Context, not prompts](https://butschster.medium.com/context-not-prompts-the-missing-piece-in-effective-ai-assisted-development-080f90174953)
3. [How to Maintain Documentation for Evolving Software Architecture](https://www.linkedin.com/advice/3/what-best-strategies-maintaining-documentation-pyodf)
4. [Cursor Documentation – Project & Nested Rules for Context](https://docs.cursor.com/context/rules)
