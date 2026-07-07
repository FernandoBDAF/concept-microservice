#!/bin/sh
# Runs once on first postgres startup (docker-entrypoint-initdb.d).
# Creates the two application databases per CONTRACTS.md §1.
set -e

psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" <<-EOSQL
	CREATE USER auth_user WITH PASSWORD 'auth_password';
	CREATE DATABASE api_db OWNER postgres;
	CREATE DATABASE auth_db OWNER auth_user;
	GRANT ALL PRIVILEGES ON DATABASE auth_db TO auth_user;
EOSQL
