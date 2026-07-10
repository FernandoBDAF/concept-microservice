# Metadata Template

-> IMPORTANT: Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined" or remove the section.

## Primary Purpose and Main Goals

### Primary Purpose

This template provides a structured approach to implementing metadata in microservices documentation, ensuring consistent and comprehensive information about each document's content, relationships, and context.

### Main Goals

1. Standardize metadata structure
2. Enhance document context
3. Improve searchability
4. Facilitate navigation
5. Enable automated processing

## Metadata Types

### 1. Document Metadata

```yaml
# document-metadata.yaml
metadata:
  document:
    title: "Service API Documentation"
    type: "api"
    category: "core"
    status: "active"
    version: "1.0"
    last_updated: "YYYY-MM-DD"
    owner: "Team Name"
    tags:
      - api
      - service
      - documentation
    dependencies:
      - dependency-service-1
      - dependency-service-2
    relationships:
      - related-service-1
      - related-service-2
```

### 2. Content Metadata

```yaml
# content-metadata.yaml
metadata:
  content:
    language: "en"
    format: "markdown"
    sections:
      - overview
      - authentication
      - endpoints
      - models
      - examples
    complexity: "intermediate"
    prerequisites:
      - prerequisite-1
      - prerequisite-2
    estimated_read_time: "30 minutes"
    target_audience:
      - developers
      - api-consumers
```

### 3. Technical Metadata

```yaml
# technical-metadata.yaml
metadata:
  technical:
    service: "service-name"
    environment:
      - development
      - staging
      - production
    technologies:
      - language
      - protocol
      - database
    dependencies:
      - dependency-1
      - dependency-2
      - dependency-3
    performance:
      - latency: "<100ms"
      - throughput: "1000 req/s"
    security:
      - authentication
      - authorization
      - encryption
```

## Implementation Guidelines

### 1. Metadata Structure

```yaml
# metadata-structure.yaml
metadata:
  required:
    document:
      - title
      - type
      - category
      - status
      - version
    content:
      - language
      - format
      - sections
    technical:
      - service
      - environment
      - technologies
  optional:
    document:
      - owner
      - tags
      - dependencies
    content:
      - complexity
      - prerequisites
      - target_audience
    technical:
      - performance
      - security
      - monitoring
```

### 2. Metadata Validation

```markdown
# Metadata Check

## Required Fields

- [ ] Document Title
- [ ] Document Type
- [ ] Document Category
- [ ] Document Status
- [ ] Document Version
- [ ] Content Language
- [ ] Content Format
- [ ] Service Name
- [ ] Environment
- [ ] Technologies

## Optional Fields

- [ ] Document Owner
- [ ] Document Tags
- [ ] Content Complexity
- [ ] Prerequisites
- [ ] Performance Metrics
- [ ] Security Requirements
```

### 3. Metadata Maintenance

```markdown
# Metadata Update

## Regular Checks

1. Verify required fields
2. Update optional fields
3. Validate relationships
4. Check dependencies
5. Update timestamps

## Documentation Updates

1. Review metadata
2. Update fields
3. Validate structure
4. Update references
5. Document changes
```

## Best Practices

### 1. Metadata Creation

- Use consistent structure
- Include all required fields
- Add relevant optional fields
- Validate metadata
- Update regularly

### 2. Metadata Management

- Regular validation
- Field updates
- Structure maintenance
- Version control
- Change tracking

### 3. Metadata Documentation

- Document standards
- Explain fields
- Update guidelines
- Maintain logs
- Track changes

## Implementation Steps

### 1. Metadata Analysis

1. Review existing documents
2. Identify metadata needs
3. Create metadata structure
4. Validate requirements
5. Plan implementation

### 2. Metadata Implementation

1. Create templates
2. Add metadata
3. Validate structure
4. Update documents
5. Document changes

### 3. Metadata Maintenance

1. Regular reviews
2. Field updates
3. Structure validation
4. Documentation updates
5. Change tracking
