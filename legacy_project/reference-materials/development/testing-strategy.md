# Testing Strategy

## Overview

This document outlines the testing strategy for the Profile Service Microservices architecture, with a focus on ensuring reliability, performance, and security across all services. Our testing approach follows a pyramid structure, emphasizing unit tests at the base, followed by integration, end-to-end, and performance tests.

## Test Categories

### 1. Unit Tests

#### Profile Service

```go
// Example unit test for ProfileService
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
                    Get(gomock.Any(), "123").
                    Return(&Profile{ID: "123"}, nil)
            },
            want:    &Profile{ID: "123"},
            wantErr: false,
        },
        {
            name: "cache miss, success from db",
            id:   "123",
            mock: func(repo *MockRepository, cache *MockCache) {
                cache.EXPECT().
                    Get(gomock.Any(), "123").
                    Return(nil, cache.ErrNotFound)
                repo.EXPECT().
                    Get(gomock.Any(), "123").
                    Return(&Profile{ID: "123"}, nil)
                cache.EXPECT().
                    Set(gomock.Any(), "123", &Profile{ID: "123"}).
                    Return(nil)
            },
            want:    &Profile{ID: "123"},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            repo := NewMockRepository(ctrl)
            cache := NewMockCache(ctrl)
            logger := NewMockLogger(ctrl)

            tt.mock(repo, cache)

            s := NewProfileService(repo, cache, logger)
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

#### Storage Service

```go
// Example unit test for StorageService
func TestStorageService_StoreFile(t *testing.T) {
    tests := []struct {
        name    string
        file    *File
        mock    func(*MockRepository)
        wantErr bool
    }{
        {
            name: "successful store",
            file: &File{Name: "test.txt", Size: 1024},
            mock: func(repo *MockRepository) {
                repo.EXPECT().
                    Store(gomock.Any(), &File{Name: "test.txt", Size: 1024}).
                    Return(nil)
            },
            wantErr: false,
        },
        {
            name: "repository error",
            file: &File{Name: "test.txt", Size: 1024},
            mock: func(repo *MockRepository) {
                repo.EXPECT().
                    Store(gomock.Any(), &File{Name: "test.txt", Size: 1024}).
                    Return(errors.New("storage error"))
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            repo := NewMockRepository(ctrl)
            logger := NewMockLogger(ctrl)

            tt.mock(repo)

            s := NewStorageService(repo, logger)
            err := s.StoreFile(context.Background(), tt.file)

            if (err != nil) != tt.wantErr {
                t.Errorf("StoreFile() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### 2. Integration Tests

#### Service Communication

```go
// Example integration test for service communication
func TestProfileService_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Setup test environment
    ctx := context.Background()
    db := setupTestDB(t)
    cache := setupTestCache(t)
    logger := setupTestLogger(t)

    // Create service
    service := NewProfileService(db, cache, logger)

    // Test profile creation
    profile := &Profile{
        ID:   "test-123",
        Name: "Test User",
    }

    err := service.CreateProfile(ctx, profile)
    if err != nil {
        t.Fatalf("failed to create profile: %v", err)
    }

    // Test profile retrieval
    got, err := service.GetProfile(ctx, profile.ID)
    if err != nil {
        t.Fatalf("failed to get profile: %v", err)
    }

    if !reflect.DeepEqual(got, profile) {
        t.Errorf("GetProfile() = %v, want %v", got, profile)
    }

    // Cleanup
    cleanupTestDB(t, db)
    cleanupTestCache(t, cache)
}
```

### 3. End-to-End Tests

#### User Flow Tests

```go
// Example end-to-end test for user flow
func TestUserFlow_E2E(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping e2e test")
    }

    // Setup test environment
    ctx := context.Background()
    client := setupTestClient(t)

    // Test user registration
    user := &User{
        Email:    "test@example.com",
        Password: "test123",
    }

    token, err := client.Register(ctx, user)
    if err != nil {
        t.Fatalf("failed to register user: %v", err)
    }

    // Test profile creation
    profile := &Profile{
        Name: "Test User",
    }

    err = client.CreateProfile(ctx, token, profile)
    if err != nil {
        t.Fatalf("failed to create profile: %v", err)
    }

    // Test profile retrieval
    got, err := client.GetProfile(ctx, token, profile.ID)
    if err != nil {
        t.Fatalf("failed to get profile: %v", err)
    }

    if !reflect.DeepEqual(got, profile) {
        t.Errorf("GetProfile() = %v, want %v", got, profile)
    }

    // Cleanup
    cleanupTestUser(t, client, user.Email)
}
```

### 4. Performance Tests

#### Load Testing

```go
// Example performance test
func BenchmarkProfileService_GetProfile(b *testing.B) {
    // Setup
    ctx := context.Background()
    service := setupBenchmarkService(b)

    // Run benchmark
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := service.GetProfile(ctx, "test-123")
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## Test Environment

### Local Development

```yaml
# docker-compose.test.yml
version: "3.8"

services:
  postgres:
    image: postgres:14-alpine
    environment:
      POSTGRES_USER: test_user
      POSTGRES_PASSWORD: test_pass
      POSTGRES_DB: test_db
    ports:
      - "5432:5432"

  redis:
    image: redis:alpine
    ports:
      - "6379:6379"

  profile-service:
    build:
      context: .
      dockerfile: Dockerfile.test
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      REDIS_HOST: redis
      REDIS_PORT: 6379
    depends_on:
      - postgres
      - redis
```

### Kubernetes Environment

```yaml
# test-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service-test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: profile-service-test
  template:
    metadata:
      labels:
        app: profile-service-test
    spec:
      containers:
        - name: profile-service
          image: profile-service:test
          env:
            - name: DB_HOST
              value: postgres-test
            - name: REDIS_HOST
              value: redis-test
```

## Test Data Management

### Test Fixtures

```go
// Example test fixtures
var testProfiles = []*Profile{
    {
        ID:   "test-1",
        Name: "Test User 1",
    },
    {
        ID:   "test-2",
        Name: "Test User 2",
    },
}

var testUsers = []*User{
    {
        Email:    "test1@example.com",
        Password: "test123",
    },
    {
        Email:    "test2@example.com",
        Password: "test456",
    },
}
```

## Continuous Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/test.yml
name: Test

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Set up Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21

      - name: Run unit tests
        run: go test -v ./... -short

      - name: Run integration tests
        run: go test -v ./... -tags=integration

      - name: Run e2e tests
        run: go test -v ./... -tags=e2e

      - name: Run performance tests
        run: go test -v ./... -bench=. -benchmem
```

## Test Coverage Requirements

- Unit tests: > 80% coverage
- Integration tests: > 60% coverage
- E2E tests: Critical paths covered
- Performance tests: All benchmarks passing

## Best Practices

1. **Test Organization**

   - Group tests by functionality
   - Use descriptive test names
   - Follow AAA pattern (Arrange, Act, Assert)
   - Keep tests independent

2. **Test Data**

   - Use fixtures for common data
   - Clean up test data
   - Use unique identifiers
   - Avoid test interdependencies

3. **Test Performance**

   - Use test suites
   - Parallelize tests
   - Mock external dependencies
   - Use appropriate timeouts

4. **Test Maintenance**
   - Regular test review
   - Update test documentation
   - Monitor test coverage
   - Refactor test code

## References

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Test-Driven Development](https://en.wikipedia.org/wiki/Test-driven_development)
- [Integration Testing Best Practices](https://martinfowler.com/articles/microservice-testing/)
- [Performance Testing Guide](https://www.nginx.com/blog/testing-the-performance-of-nginx-and-nginx-plus/)
