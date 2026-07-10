INITIAL CONTEXT FOR LLM - never change the context-----------------------------
-> THIS SECTION IS A GUIDELINE TO THE LLM CONSIDER BEFORE WORKING IN THIS FILE, DO NOT CHANGE THIS

-> GOES OF THE TRACKER&MANAGER FILE:

- This file serves as the development progress tracker and project management tool
- It should track:
  - Development phases and milestones
  - Task status and progress
  - Dependencies and blockers
  - Technical decisions and their rationale
  - Questions and clarifications needed
- This is the primary reference for understanding the development progress
- This file should be in sync with the `/README.md` where technical implementation details are documented
- While README.md focuses on "how" and "why", this file focuses on "what" and "when"

-> CONSIDERER BEFORE UPDATING THIS FILE:

- Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined".
- The changes in this file need to be incremental, so update tasks as they are completed, reorganize them or add new tasks but do not remove any of the previous one. If something needs to be removed, add a note instead.
- Update informations that you confidentilly have knowlegde, they should not be guesses.
- If there are questions or uncertanty add comments asking for clarification instead.
- Check the `microservices/docs` folder for comprehensive project details, including architecture, development guidelines, and integration points. This will help in making informed decisions, haver better context and updates to the development plan. Always compare the implementation of this project with the plan described in the docs and whenever there are inconsistancies, add comments.
- Do not forget to be LLM focus, so because this will be used
- For LLM-specific guidelines and patterns, refer to [LLM Integration Guide](../../docs/llm/README.md)

---

# [Service Name] Development Tracker

## Current Status

### Phase

- Current Phase: [Phase Name]
- Status: [In Progress/Completed/Blocked]
- Next Steps: [List immediate next steps]
- Blockers: [List any blockers]
- Dependencies: [List dependencies]

### Implementation Progress

#### Core Features

- [ ] Feature 1
  - [ ] Subtask 1.1
  - [ ] Subtask 1.2
- [ ] Feature 2
  - [ ] Subtask 2.1
  - [ ] Subtask 2.2

#### Infrastructure

- [ ] Kubernetes Configuration
  - [ ] Deployment manifests
  - [ ] Service definitions
  - [ ] Resource limits
- [ ] Docker Setup
  - [ ] Dockerfile
  - [ ] docker-compose.yml
  - [ ] Build optimization

#### Testing

- [ ] Unit Tests
  - [ ] Core functionality
  - [ ] Edge cases
- [ ] Integration Tests
  - [ ] Service integration
  - [ ] API endpoints
- [ ] Performance Tests
  - [ ] Load testing
  - [ ] Stress testing

#### Documentation

- [ ] API Documentation
  - [ ] OpenAPI spec
  - [ ] Request/response examples
- [ ] Deployment Guide
  - [ ] Setup instructions
  - [ ] Configuration guide
- [ ] Development Guide
  - [ ] Local setup
  - [ ] Testing procedures

## Development Phases

### Phase 1: Foundation

- [ ] Project Structure
  - [ ] Directory organization
  - [ ] Base configurations
- [ ] Development Environment
  - [ ] Local setup
  - [ ] Development tools
- [ ] Basic Infrastructure
  - [ ] Docker configuration
  - [ ] Kubernetes manifests

### Phase 2: Core Implementation

- [ ] Core Features
  - [ ] Feature implementation
  - [ ] Error handling
- [ ] Service Integration
  - [ ] API endpoints
  - [ ] Service communication
- [ ] Testing Framework
  - [ ] Test setup
  - [ ] Test coverage

### Phase 3: Enhancement

- [ ] Performance Optimization
  - [ ] Code optimization
  - [ ] Resource usage
- [ ] Security Implementation
  - [ ] Authentication
  - [ ] Authorization
- [ ] Monitoring Setup
  - [ ] Metrics collection
  - [ ] Health checks

### Phase 4: Production Readiness

- [ ] Documentation
  - [ ] API documentation
  - [ ] Deployment guides
- [ ] Production Configuration
  - [ ] Environment setup
  - [ ] Resource management
- [ ] Monitoring
  - [ ] Alerting
  - [ ] Logging

## Task Management

### Current Tasks

- [ ] Task 1
  - Priority: [High/Medium/Low]
  - Status: [In Progress/Blocked/Completed]
  - Dependencies: [List dependencies]
- [ ] Task 2
  - Priority: [High/Medium/Low]
  - Status: [In Progress/Blocked/Completed]
  - Dependencies: [List dependencies]

### Completed Tasks

- [x] Task 1
  - Completion Date: [Date]
  - Notes: [Any relevant notes]
- [x] Task 2
  - Completion Date: [Date]
  - Notes: [Any relevant notes]

### Upcoming Tasks

- [ ] Task 1
  - Priority: [High/Medium/Low]
  - Dependencies: [List dependencies]
- [ ] Task 2
  - Priority: [High/Medium/Low]
  - Dependencies: [List dependencies]

## Notes and Decisions

### Technical Decisions

- Decision 1
  - Context: [Why this decision was made]
  - Impact: [Impact on the project]
  - Alternatives Considered: [List alternatives]
- Decision 2
  - Context: [Why this decision was made]
  - Impact: [Impact on the project]
  - Alternatives Considered: [List alternatives]

### Questions and Clarifications

- Question 1
  - Context: [Background of the question]
  - Impact: [Impact on development]
  - Status: [Open/Resolved]
- Question 2
  - Context: [Background of the question]
  - Impact: [Impact on development]
  - Status: [Open/Resolved]

## History

### Recent Updates

- [Date] - [Update description]
- [Date] - [Update description]

### Major Milestones

- [Date] - [Milestone description]
- [Date] - [Milestone description]
