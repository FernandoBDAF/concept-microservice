# Go Testing Frameworks Guide

> *Migrated from legacy_project/reference-materials/development/tools/testing-frameworks.md*

## Overview

This guide covers the testing frameworks and tools used in our architecture for writing and executing tests. We primarily use Go's built-in testing package along with several popular testing libraries to ensure comprehensive test coverage and maintainable test code.

## Core Testing Tools

### 1. Go Testing Package

The standard `testing` package provides the foundation for our tests:

```go
// Basic test structure
func TestProfileService(t *testing.T) {
    t.Run("subtest name", func(t *testing.T) {
        // Test implementation
    })
}

// Table-driven tests
func TestProfileValidation(t *testing.T) {
    tests := []struct {
        name    string
        profile *Profile
        wantErr bool
    }{
        {
            name: "valid profile",
            profile: &Profile{
                FirstName: "John",
                LastName:  "Doe",
                Email:     "john@example.com",
            },
            wantErr: false,
        },
        {
            name: "invalid email",
            profile: &Profile{
                FirstName: "John",
                LastName:  "Doe",
                Email:     "invalid-email",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.profile.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 2. Testify

We use Testify for assertions and mocking:

```go
// Using Testify assertions
func TestProfileService_GetProfile(t *testing.T) {
    assert := assert.New(t)
    require := require.New(t)

    service := NewProfileService(mockRepo, mockCache, nil, logger)
    profile, err := service.GetProfile(ctx, "123")

    require.NoError(err)
    assert.NotNil(profile)
    assert.Equal("123", profile.ID)
}

// Using Testify mocks
func TestProfileService_CreateProfile(t *testing.T) {
    mockRepo := new(MockRepository)
    mockCache := new(MockCache)

    mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*Profile")).
        Return(nil)
    mockCache.On("SetProfile", mock.Anything, mock.Anything).
        Return(nil)

    service := NewProfileService(mockRepo, mockCache, nil, logger)
    err := service.CreateProfile(ctx, &Profile{FirstName: "Test"})

    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

### 3. GoMock

For more complex mocking scenarios:

```go
// Using GoMock
func TestProfileService_UpdateProfile(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := NewMockRepository(ctrl)
    mockCache := NewMockCache(ctrl)

    mockRepo.EXPECT().
        Update(gomock.Any(), gomock.Any()).
        Return(nil)
    mockCache.EXPECT().
        DeleteProfile(gomock.Any(), gomock.Any()).
        Return(nil)

    service := NewProfileService(mockRepo, mockCache, nil, logger)
    err := service.UpdateProfile(ctx, &Profile{ID: "123"})

    assert.NoError(t, err)
}
```

## Test Utilities

### 1. Test Containers

For integration tests requiring external services:

```go
// Using testcontainers-go
func TestProfileService_Integration(t *testing.T) {
    ctx := context.Background()

    // Start PostgreSQL container
    postgres, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:15-alpine"),
        postgres.WithDatabase("test_db"),
        postgres.WithUsername("test_user"),
        postgres.WithPassword("test_pass"),
    )
    require.NoError(t, err)
    defer postgres.Terminate(ctx)

    // Start Redis container
    redisC, err := redis.RunContainer(ctx,
        testcontainers.WithImage("redis:7-alpine"),
    )
    require.NoError(t, err)
    defer redisC.Terminate(ctx)

    // Get connection details
    pgHost, _ := postgres.Host(ctx)
    pgPort, _ := postgres.MappedPort(ctx, "5432")
    redisHost, _ := redisC.Host(ctx)
    redisPort, _ := redisC.MappedPort(ctx, "6379")

    // Initialize service with container connections
    db, _ := sqlx.Open("postgres", fmt.Sprintf(
        "postgres://test_user:test_pass@%s:%d/test_db?sslmode=disable",
        pgHost, pgPort.Int()))
    
    redisClient := redis.NewClient(&redis.Options{
        Addr: fmt.Sprintf("%s:%d", redisHost, redisPort.Int()),
    })

    service := NewProfileService(
        NewProfileRepository(db),
        NewCache(redisClient, time.Hour),
        nil,
        zap.NewNop(),
    )

    // Run tests
    // ...
}
```

### 2. HTTP Testing

For API endpoint testing:

```go
// Using httptest with Gin
func TestProfileHandler_GetProfile(t *testing.T) {
    gin.SetMode(gin.TestMode)
    
    mockService := new(MockProfileService)
    mockService.On("GetProfile", mock.Anything, "123").
        Return(&Profile{ID: "123", FirstName: "Test"}, nil)

    handler := NewProfileHandler(mockService)
    
    router := gin.New()
    router.GET("/profiles/:id", handler.GetProfile)

    req := httptest.NewRequest("GET", "/profiles/123", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    
    var profile Profile
    err := json.Unmarshal(w.Body.Bytes(), &profile)
    assert.NoError(t, err)
    assert.Equal(t, "123", profile.ID)
}
```

## Best Practices

1. **Test Organization**

   - Group related tests together
   - Use descriptive test names
   - Follow the AAA pattern (Arrange, Act, Assert)
   - Keep tests independent and isolated

2. **Mocking Strategy**

   - Mock external dependencies
   - Use interfaces for better testability
   - Keep mocks simple and focused
   - Verify mock expectations

3. **Test Data Management**

   - Use test fixtures
   - Clean up test data
   - Use unique identifiers
   - Avoid test interdependencies

4. **Performance Considerations**
   - Use test suites for organization
   - Parallelize tests when possible
   - Use appropriate timeouts
   - Monitor test execution time

## Common Issues and Solutions

1. **Flaky Tests**

   - Problem: Tests failing intermittently
   - Solution: Add proper synchronization, use test containers

2. **Slow Tests**

   - Problem: Tests taking too long to run
   - Solution: Use mocks, parallelize tests, optimize setup

3. **Complex Test Setup**
   - Problem: Tests difficult to maintain
   - Solution: Use test helpers, fixtures, and proper organization

## References

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Testify Documentation](https://github.com/stretchr/testify)
- [GoMock Documentation](https://github.com/golang/mock)
- [TestContainers Documentation](https://golang.testcontainers.org/)
