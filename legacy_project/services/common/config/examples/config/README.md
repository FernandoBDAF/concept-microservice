# Configuration Package Example

This example demonstrates the usage of the configuration package with all its features:

- Multiple format support (JSON, YAML, TOML)
- Environment variable overrides
- Configuration validation
- Hot-reloading
- Encryption
- Versioning
- Migration utilities

## Running the Example

1. Make sure you have Go 1.21 or later installed
2. Navigate to the example directory:
   ```bash
   cd examples/config
   ```
3. Run the example:
   ```bash
   go run main.go
   ```

## What to Expect

The example will:

1. Create a temporary configuration file
2. Load and validate the configuration
3. Encrypt sensitive values
4. Perform a configuration migration
5. Create a backup and migration report
6. Start watching for configuration changes
7. Simulate a configuration change after 2 seconds
8. Print the configuration values

## Output

You should see output similar to:

```
Environment: development
Version: 2.0.0
Database Address: localhost
Database Port: 5432
API Port: 8080
API Timeout: 30
Configuration reloaded: port=9090
```

## Generated Files

The example will create:

- A backup file in the `backups` directory
- A migration report in the `migration_reports` directory

These files will be automatically cleaned up when the program exits.

## Notes

- The example uses a temporary file for demonstration purposes
- In a real application, you would use a permanent configuration file
- The encryption key is hardcoded for demonstration; in production, use a secure key management system
- The example includes custom validation rules for the database and API settings
