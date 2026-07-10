# Development Best Practices

> *Migrated from legacy_project/reference-materials/development/best-practices.md*

## Overview

This document outlines the best practices for development in the Profile Service system, providing comprehensive guidelines for code organization, development workflow, testing, and deployment.

## Code Organization

### 1. Project Structure

```yaml
project_structure:
  root:
    - cmd/ # Application entry points
    - internal/ # Private application code
    - pkg/ # Public library code
    - api/ # API definitions
    - configs/ # Configuration files
    - deployments/ # Deployment configurations
    - docs/ # Documentation
    - scripts/ # Build and utility scripts
    - test/ # Additional test files
```

#### Guidelines

1. **Directory Organization**

   - Use clear, descriptive directory names
   - Group related functionality
   - Maintain consistent structure
   - Follow language conventions

2. **File Organization**

   - One primary purpose per file
   - Clear file naming conventions
   - Consistent file extensions
   - Proper file permissions

3. **Module Organization**
   - Clear module boundaries
   - Explicit dependencies
   - Minimal coupling
   - Maximum cohesion

### 2. Code Style

```yaml
code_style:
  formatting:
    - use_consistent_indentation
    - follow_language_conventions
    - maintain_line_length_limits
    - organize_imports

  naming:
    - use_clear_descriptive_names
    - follow_language_conventions
    - be_consistent
    - avoid_abbreviations

  documentation:
    - document_public_apis
    - use_clear_comments
    - maintain_readme_files
    - update_documentation
```

### 3. Testing

```yaml
testing:
  unit_tests:
    - test_individual_components
    - mock_dependencies
    - verify_behavior
    - maintain_coverage

  integration_tests:
    - test_component_interaction
    - verify_data_flow
    - check_error_handling
    - validate_contracts

  e2e_tests:
    - test_complete_workflows
    - verify_user_journeys
    - check_system_integration
    - validate_requirements
```

### 4. Version Control

```yaml
version_control:
  branching:
    - main: production_ready
    - develop: integration_branch
    - feature/*: new_features
    - bugfix/*: bug_fixes
    - release/*: release_preparation

  commits:
    - use_clear_messages
    - reference_issues
    - keep_changes_focused
    - follow_conventions

  pull_requests:
    - provide_context
    - include_tests
    - update_documentation
    - request_reviews
```

## Development Workflow

### 1. Local Development

```yaml
local_development:
  environment:
    - use_docker
    - configure_ide
    - setup_tools
    - manage_dependencies

  testing:
    - run_tests_locally
    - debug_issues
    - verify_changes
    - check_quality

  quality:
    - run_linters
    - check_formatting
    - verify_documentation
    - test_coverage
```

### 2. Continuous Integration

```yaml
continuous_integration:
  pipeline:
    - build_artifacts
    - run_tests
    - check_quality
    - deploy_staging

  quality_checks:
    - code_review
    - security_scan
    - performance_test
    - integration_test

  deployment:
    - automated_deployment
    - environment_management
    - configuration_management
    - monitoring_setup
```

### 3. Code Review

```yaml
code_review:
  checklist:
    - code_quality
    - test_coverage
    - documentation
    - security

  feedback:
    - be_constructive
    - provide_examples
    - suggest_improvements
    - verify_changes

  process:
    - assign_reviewers
    - set_deadlines
    - track_changes
    - verify_approval
```

## Implementation Examples

### 1. Service Implementation

```go
// Example service implementation with direct infrastructure access
type ProfileService struct {
    repository *postgres.ProfileRepository  // Direct PostgreSQL
    cache      *redis.Cache                 // Direct Redis
    publisher  *rabbitmq.Publisher          // Direct RabbitMQ
    logger     *zap.Logger
}

func NewProfileService(
    repo *postgres.ProfileRepository,
    cache *redis.Cache,
    publisher *rabbitmq.Publisher,
    logger *zap.Logger,
) *ProfileService {
    return &ProfileService{
        repository: repo,
        cache:      cache,
        publisher:  publisher,
        logger:     logger,
    }
}

func (s *ProfileService) GetProfile(ctx context.Context, id string) (*Profile, error) {
    // Check cache first (direct Redis access)
    if profile, err := s.cache.GetProfile(ctx, id); err == nil {
        return profile, nil
    }

    // Get from repository (direct PostgreSQL access)
    profile, err := s.repository.Get(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get profile: %w", err)
    }

    // Update cache
    if err := s.cache.SetProfile(ctx, profile); err != nil {
        s.logger.Warn("failed to update cache", zap.Error(err))
    }

    return profile, nil
}
```

### 2. Testing Example

```go
// Example test implementation
func TestProfileService_GetProfile(t *testing.T) {
    tests := []struct {
        name    string
        id      string
        mock    func(*MockRepository, *MockCache)
        want    *Profile
        wantErr bool
    }{
        {
            name: "success from cache",
            id:   "123",
            mock: func(repo *MockRepository, cache *MockCache) {
                cache.EXPECT().
                    GetProfile(gomock.Any(), "123").
                    Return(&Profile{ID: "123"}, nil)
            },
            want:    &Profile{ID: "123"},
            wantErr: false,
        },
        // Add more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            repo := NewMockRepository(ctrl)
            cache := NewMockCache(ctrl)
            logger := zap.NewNop()

            tt.mock(repo, cache)

            s := NewProfileService(repo, cache, nil, logger)
            got, err := s.GetProfile(context.Background(), tt.id)

            if (err != nil) != tt.wantErr {
                t.Errorf("GetProfile() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("GetProfile() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Resources

- [Logging Best Practices](logging-best-practices.md)
- [Error Handling Best Practices](error-handling-best-practices.md)
- [API Design Best Practices](api-design-best-practices.md)
- [Database Best Practices](database-best-practices.md)
- [Security Best Practices](security-best-practices.md)

## Maintenance

- Regular review of practices
- Update documentation
- Track improvements
- Gather feedback
- Monitor effectiveness
- Adjust as needed

## References

- [Go Best Practices](https://golang.org/doc/effective_go)
- [Microservices Best Practices](https://microservices.io/patterns/index.html)
- [Clean Code Principles](https://clean-code-developer.com/)
- [SOLID Principles](https://en.wikipedia.org/wiki/SOLID)
