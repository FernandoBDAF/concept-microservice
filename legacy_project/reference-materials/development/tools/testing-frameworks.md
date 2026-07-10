# Go Testing Frameworks Guide

## Overview

This guide covers the testing frameworks and tools used in our microservices architecture for writing and executing tests. We primarily use Go's built-in testing package along with several popular testing libraries to ensure comprehensive test coverage and maintainable test code.

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
                Name: "John Doe",
                Email: "john@example.com",
            },
            wantErr: false,
        },
        {
            name: "invalid email",
            profile: &Profile{
                Name: "John Doe",
                Email: "invalid-email",
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

    service := NewProfileService(mockRepo, mockCache)
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
    mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything).
        Return(nil)

    service := NewProfileService(mockRepo, mockCache)
    err := service.CreateProfile(ctx, &Profile{Name: "Test"})

    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
    mockCache.AssertExpectations(t)
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
        Delete(gomock.Any(), gomock.Any()).
        Return(nil)

    service := NewProfileService(mockRepo, mockCache)
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
        testcontainers.WithImage("postgres:14-alpine"),
        postgres.WithDatabase("test_db"),
        postgres.WithUsername("test_user"),
        postgres.WithPassword("test_pass"),
    )
    require.NoError(t, err)
    defer postgres.Terminate(ctx)

    // Get connection details
    host, err := postgres.Host(ctx)
    require.NoError(t, err)
    port, err := postgres.MappedPort(ctx, "5432")
    require.NoError(t, err)

    // Initialize service with container connection
    service := NewProfileService(
        NewPostgresRepository(fmt.Sprintf("postgres://%s:%s@%s:%d/test_db",
            "test_user", "test_pass", host, port.Int())),
        NewRedisCache(),
    )

    // Run tests
    // ...
}
```

### 2. HTTP Testing

For API endpoint testing:

```go
// Using httptest
func TestProfileHandler_GetProfile(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock response
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(Profile{ID: "123", Name: "Test User"})
    }))
    defer server.Close()

    // Create client
    client := &http.Client{}
    req, err := http.NewRequest("GET", server.URL+"/profiles/123", nil)
    require.NoError(t, err)

    // Make request
    resp, err := client.Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    // Assert response
    assert.Equal(t, http.StatusOK, resp.StatusCode)
    var profile Profile
    err = json.NewDecoder(resp.Body).Decode(&profile)
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
