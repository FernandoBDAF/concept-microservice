# Database Migrations

This directory contains database migrations for the Profile Storage Service.

## Schema Overview

### Profiles Table

- Primary table for storing user profiles
- Contains basic profile information
- Uses UUID as primary key
- Includes timestamps for creation and updates

Fields:

- `id`: UUID (Primary Key)
- `first_name`: VARCHAR(100)
- `last_name`: VARCHAR(100)
- `email`: VARCHAR(255) (Unique)
- `phone`: VARCHAR(20)
- `created_at`: TIMESTAMP WITH TIME ZONE
- `updated_at`: TIMESTAMP WITH TIME ZONE

### Addresses Table

- Stores address information for profiles
- Links to profiles via foreign key
- Supports multiple addresses per profile
- Includes primary address flag

Fields:

- `id`: UUID (Primary Key)
- `profile_id`: UUID (Foreign Key)
- `street`: VARCHAR(255)
- `city`: VARCHAR(100)
- `state`: VARCHAR(100)
- `country`: VARCHAR(100)
- `postal_code`: VARCHAR(20)
- `is_primary`: BOOLEAN
- `created_at`: TIMESTAMP WITH TIME ZONE
- `updated_at`: TIMESTAMP WITH TIME ZONE

### Contacts Table

- Stores additional contact information
- Links to profiles via foreign key
- Supports multiple contact methods
- Includes contact type and primary flag

Fields:

- `id`: UUID (Primary Key)
- `profile_id`: UUID (Foreign Key)
- `type`: VARCHAR(50)
- `value`: VARCHAR(255)
- `is_primary`: BOOLEAN
- `created_at`: TIMESTAMP WITH TIME ZONE
- `updated_at`: TIMESTAMP WITH TIME ZONE

## Indexes

1. `idx_profiles_email`: Index on email field for faster lookups
2. `idx_addresses_profile_id`: Index on profile_id for faster joins
3. `idx_contacts_profile_id`: Index on profile_id for faster joins

## Triggers

- `update_updated_at_column`: Automatically updates the `updated_at` timestamp
- Applied to all tables for consistent timestamp management

## Migration Files

1. `000001_init_schema.up.sql`: Creates initial schema

   - Creates tables
   - Sets up indexes
   - Creates triggers
   - Sets up constraints

2. `000001_init_schema.down.sql`: Reverts schema changes
   - Drops triggers
   - Drops indexes
   - Drops tables
   - Drops functions

## Running Migrations

To run migrations:

```bash
# Apply migrations
migrate -path ./migrations -database "postgresql://user:password@localhost:5432/profiles?sslmode=disable" up

# Revert migrations
migrate -path ./migrations -database "postgresql://user:password@localhost:5432/profiles?sslmode=disable" down
```

## Notes

- All tables use UUID as primary keys
- Timestamps are in UTC
- Foreign keys have ON DELETE CASCADE
- Email addresses are unique
- Indexes are created for frequently queried fields
- Triggers maintain updated_at timestamps
