# Utils Package

A collection of utility functions and helpers for Go microservices, providing common functionality and convenience methods.

## Overview

The utils package provides a set of utility functions and helpers that are commonly used across microservices. It includes string manipulation, time handling, file operations, and other useful utilities.

## Features

- String utilities
- Time utilities
- File operations
- HTTP helpers
- JSON utilities
- Validation helpers
- Error handling
- Testing utilities
- Debug helpers
- Performance utilities

## Installation

```bash
go get github.com/your-org/common/utils
```

## Quick Start

```go
package main

import (
    "github.com/your-org/common/utils"
)

func main() {
    // Generate UUID
    uuid := utils.GenerateUUID()

    // Format time
    formatted := utils.FormatTime(time.Now())

    // Parse JSON
    data := map[string]interface{}{"key": "value"}
    jsonStr, err := utils.ToJSON(data)
    if err != nil {
        // Handle error
    }
}
```

## String Utilities

```go
// Generate UUID
uuid := utils.GenerateUUID()

// Generate random string
random := utils.RandomString(10)

// Truncate string
truncated := utils.TruncateString("long string", 5)

// Convert to snake case
snake := utils.ToSnakeCase("camelCase")

// Convert to camel case
camel := utils.ToCamelCase("snake_case")

// Check if string is empty
isEmpty := utils.IsEmpty("")

// Check if string is blank
isBlank := utils.IsBlank("   ")
```

## Time Utilities

```go
// Format time
formatted := utils.FormatTime(time.Now())

// Parse time
parsed, err := utils.ParseTime("2006-01-02 15:04:05", "2023-01-01 12:00:00")
if err != nil {
    // Handle error
}

// Get start of day
startOfDay := utils.StartOfDay(time.Now())

// Get end of day
endOfDay := utils.EndOfDay(time.Now())

// Format duration
formatted := utils.FormatDuration(time.Hour * 2)
```

## File Operations

```go
// Read file
content, err := utils.ReadFile("file.txt")
if err != nil {
    // Handle error
}

// Write file
err := utils.WriteFile("file.txt", []byte("content"))
if err != nil {
    // Handle error
}

// Check if file exists
exists := utils.FileExists("file.txt")

// Create directory
err := utils.CreateDir("dir")
if err != nil {
    // Handle error
}

// List files
files, err := utils.ListFiles("dir")
if err != nil {
    // Handle error
}
```

## HTTP Helpers

```go
// Make HTTP request
resp, err := utils.HTTPGet("https://api.example.com")
if err != nil {
    // Handle error
}

// Parse JSON response
var data map[string]interface{}
err = utils.ParseJSONResponse(resp, &data)
if err != nil {
    // Handle error
}

// Download file
err := utils.DownloadFile("https://example.com/file.txt", "file.txt")
if err != nil {
    // Handle error
}

// Check if URL is valid
isValid := utils.IsValidURL("https://example.com")
```

## JSON Utilities

```go
// Convert to JSON
data := map[string]interface{}{"key": "value"}
jsonStr, err := utils.ToJSON(data)
if err != nil {
    // Handle error
}

// Parse JSON
var data map[string]interface{}
err = utils.FromJSON(jsonStr, &data)
if err != nil {
    // Handle error
}

// Pretty print JSON
pretty, err := utils.PrettyJSON(data)
if err != nil {
    // Handle error
}
```

## Validation Helpers

```go
// Validate email
isValid := utils.IsValidEmail("user@example.com")

// Validate URL
isValid := utils.IsValidURL("https://example.com")

// Validate phone number
isValid := utils.IsValidPhone("+1234567890")

// Validate date
isValid := utils.IsValidDate("2023-01-01")

// Validate number
isValid := utils.IsValidNumber("123.45")
```

## Error Handling

```go
// Wrap error
err := utils.WrapError(err, "failed to process")

// Check if error is of type
isType := utils.IsErrorType(err, &CustomError{})

// Get error message
message := utils.GetErrorMessage(err)

// Log error
utils.LogError(err)

// Panic if error
utils.PanicIfError(err)
```

## Testing Utilities

```go
// Assert equal
utils.AssertEqual(t, expected, actual)

// Assert not equal
utils.AssertNotEqual(t, expected, actual)

// Assert nil
utils.AssertNil(t, value)

// Assert not nil
utils.AssertNotNil(t, value)

// Assert true
utils.AssertTrue(t, condition)

// Assert false
utils.AssertFalse(t, condition)
```

## Debug Helpers

```go
// Print debug info
utils.DebugPrint("message", data)

// Print stack trace
utils.PrintStackTrace()

// Get memory stats
stats := utils.GetMemoryStats()

// Get goroutine count
count := utils.GetGoroutineCount()

// Get CPU usage
usage := utils.GetCPUUsage()
```

## Performance Utilities

```go
// Measure execution time
duration := utils.MeasureTime(func() {
    // Code to measure
})

// Get memory usage
usage := utils.GetMemoryUsage()

// Get CPU usage
usage := utils.GetCPUUsage()

// Get disk usage
usage := utils.GetDiskUsage()

// Get network usage
usage := utils.GetNetworkUsage()
```

## Best Practices

1. **String Operations**

   - Use appropriate string functions
   - Handle empty strings
   - Validate input
   - Use constants

2. **Time Operations**

   - Use UTC for storage
   - Handle time zones
   - Validate dates
   - Use appropriate formats

3. **File Operations**

   - Handle errors
   - Close files
   - Check permissions
   - Use appropriate modes

4. **HTTP Operations**
   - Handle timeouts
   - Check status codes
   - Validate responses
   - Use appropriate methods

## Examples

### HTTP Client

```go
package main

import (
    "github.com/your-org/common/utils"
)

func main() {
    // Make HTTP request
    resp, err := utils.HTTPGet("https://api.example.com")
    if err != nil {
        // Handle error
    }

    // Parse JSON response
    var data map[string]interface{}
    err = utils.ParseJSONResponse(resp, &data)
    if err != nil {
        // Handle error
    }

    // Process data
    // ...
}
```

### File Operations

```go
package main

import (
    "github.com/your-org/common/utils"
)

func main() {
    // Read file
    content, err := utils.ReadFile("file.txt")
    if err != nil {
        // Handle error
    }

    // Process content
    processed := utils.ProcessContent(content)

    // Write file
    err = utils.WriteFile("output.txt", processed)
    if err != nil {
        // Handle error
    }
}
```

### Validation

```go
package main

import (
    "github.com/your-org/common/utils"
)

func main() {
    // Validate input
    if !utils.IsValidEmail("user@example.com") {
        // Handle invalid email
    }

    if !utils.IsValidPhone("+1234567890") {
        // Handle invalid phone
    }

    if !utils.IsValidDate("2023-01-01") {
        // Handle invalid date
    }
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

This package is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
