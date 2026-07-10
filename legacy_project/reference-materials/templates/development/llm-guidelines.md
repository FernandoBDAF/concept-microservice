# LLM Integration Guidelines

## Initial Context for LLM

### README File Guidelines

- This file serves as the technical documentation of the service, providing a comprehensive overview of the codebase
- It should document:
  - Service architecture and design decisions
  - Component structure and relationships
  - API endpoints and interfaces
  - Dependencies and integration points
  - Configuration and deployment details
- This is the primary reference for understanding the technical implementation
- This file should be in sync with the project tracker where development progress and tasks are tracked
- While the tracker focuses on "what" and "when", this file focuses on "how" and "why"

### Important Considerations

- Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined" or remove the section.
- Changes in documentation need to be incremental or to update information that you confidently have knowledge of, they should not be guesses.
- If there are questions or uncertainty, add comments asking for clarification instead.
- Check the project documentation for comprehensive details, including architecture, development guidelines, and integration points.
- Consider structuring documentation by separating components and describing how they interact with each other.
- Always maintain LLM-friendly formatting and structure.

## Documentation Structure

### Quick Reference

#### Documentation Structure

- Clear hierarchical organization
- Explicit context markers
- Maintained references
- Clear relationships

#### Code Structure

- Consistent patterns
- Context comments
- Clear boundaries
- Explicit interfaces

#### Development Process

- Structured artifacts
- Clear context
- Maintained references
- Documented decisions

## LLM Integration Protocol

### 1. Code Generation

- Use LLM for code generation with clear prompts:
  ```prompt
  # Example: Code Generation
  Generate [component] for [service] that:
  - Implements [requirements]
  - Follows [patterns]
  - Includes [features]
  - Handles [cases]
  ```

### 2. Code Validation

- Validate generated code through:
  - Code review
  - Testing
  - Security check
  - Performance validation

### 3. Documentation Updates

- Document all decisions:
  - Design choices
  - Implementation details
  - Trade-offs
  - Alternatives
- Update documentation for:
  - Code changes
  - Design updates
  - Configuration
  - Integration

### 4. Cross-References

- Maintain cross-references:
  - Update related documents
  - Verify link validity
  - Track dependencies
  - Document relationships

## Best Practices

### 1. Documentation Structure

- Separate components and their interactions
- Keep documentation dynamic and up-to-date
- Maintain clear update paths for changes
- Use consistent formatting

### 2. Information Management

- Only include verified information
- Mark uncertain information appropriately
- Add comments for clarification needs
- Track changes incrementally

### 3. Context Maintenance

- Keep documentation in sync with implementation
- Update cross-references regularly
- Maintain relationship documentation
- Track dependencies accurately

## Related Documentation

- [Development Guide](../../guides/development/guide.md)
- [Documentation Standards](../../standards/documentation.md)
- [Code Standards](../../standards/code.md)
- [Service Templates](../README.md)

## Notes

- All documentation should be LLM-friendly
- Keep information accurate and verified
- Update documentation incrementally
- Maintain clear component relationships
