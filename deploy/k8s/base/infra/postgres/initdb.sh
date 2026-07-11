#!/bin/sh
# In-cluster twin of scripts/compose/init-postgres.sh (CONTRACTS.md §1):
# same databases/users, but auth_user's password comes from the Secret via
# env instead of being hardcoded.
set -e

psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" <<-EOSQL
	CREATE USER auth_user WITH PASSWORD '$AUTH_DB_PASSWORD';
	CREATE DATABASE api_db OWNER postgres;
	CREATE DATABASE auth_db OWNER auth_user;
	GRANT ALL PRIVILEGES ON DATABASE auth_db TO auth_user;
EOSQL
