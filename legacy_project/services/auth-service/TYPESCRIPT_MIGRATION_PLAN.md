# Auth-Service TypeScript Migration Plan

## Overview

This document provides a detailed, step-by-step guide for migrating the auth-service from JavaScript to TypeScript, transforming it into a portfolio-ready reference Node.js project.

**Estimated Timeline**: 2-3 weeks  
**Effort Level**: High  
**Final Stack**: Node 22 + TypeScript 5.x + Express + Zod + Vitest + Pino + OpenAPI

---

## Table of Contents

1. [Phase 1: Foundation Setup](#phase-1-foundation-setup)
2. [Phase 2: Core Types & Interfaces](#phase-2-core-types--interfaces)
3. [Phase 3: Infrastructure Migration](#phase-3-infrastructure-migration)
4. [Phase 4: Business Logic Migration](#phase-4-business-logic-migration)
5. [Phase 5: Validation & Error Handling](#phase-5-validation--error-handling)
6. [Phase 6: Testing Setup](#phase-6-testing-setup)
7. [Phase 7: Documentation & OpenAPI](#phase-7-documentation--openapi)
8. [Phase 8: DevOps & Polish](#phase-8-devops--polish)
9. [File Migration Checklist](#file-migration-checklist)
10. [Dependencies Reference](#dependencies-reference)

---

## Phase 1: Foundation Setup

**Duration**: Day 1  
**Goal**: Set up TypeScript tooling and project structure

### 1.1 Update Node.js Version

Create `.nvmrc`:
```
22
```

### 1.2 Install TypeScript Dependencies

```bash
npm install --save-dev \
  typescript@^5.4 \
  @types/node@^22 \
  @types/express@^4 \
  @types/bcrypt@^5 \
  @types/jsonwebtoken@^9 \
  @types/uuid@^9 \
  @types/pg@^8 \
  tsx@^4 \
  rimraf@^5
```

### 1.3 Create TypeScript Configuration

Create `tsconfig.json`:
```json
{
  "$schema": "https://json.schemastore.org/tsconfig",
  "compilerOptions": {
    // Language & Environment
    "target": "ES2022",
    "lib": ["ES2022"],
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    
    // Output
    "outDir": "./dist",
    "rootDir": "./src",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    
    // Strict Type Checking
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "strictFunctionTypes": true,
    "strictBindCallApply": true,
    "strictPropertyInitialization": true,
    "noImplicitThis": true,
    "alwaysStrict": true,
    
    // Additional Checks
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    
    // Module Resolution
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    
    // Other
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "incremental": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "**/*.test.ts", "**/*.spec.ts"]
}
```

Create `tsconfig.build.json` (for production builds):
```json
{
  "extends": "./tsconfig.json",
  "exclude": ["node_modules", "dist", "**/*.test.ts", "**/*.spec.ts", "src/**/__tests__/**"]
}
```

### 1.4 Update package.json

```json
{
  "name": "auth-service",
  "version": "2.0.0",
  "description": "Modern authentication microservice with TypeScript",
  "type": "module",
  "main": "dist/server.js",
  "types": "dist/server.d.ts",
  "scripts": {
    "dev": "tsx watch src/server.ts",
    "build": "rimraf dist && tsc -p tsconfig.build.json",
    "start": "node dist/server.js",
    "typecheck": "tsc --noEmit",
    "lint": "eslint src --ext .ts",
    "lint:fix": "eslint src --ext .ts --fix",
    "test": "vitest run",
    "test:watch": "vitest",
    "test:coverage": "vitest run --coverage",
    "clean": "rimraf dist coverage"
  },
  "engines": {
    "node": ">=22.0.0"
  }
}
```

### 1.5 Create New Directory Structure

```
src/
├── config/
│   ├── index.ts              # Main config export
│   ├── env.ts                # Environment validation with Zod
│   └── database.ts           # Database configuration
├── types/
│   ├── index.ts              # Barrel export
│   ├── user.types.ts         # User domain types
│   ├── auth.types.ts         # Auth domain types
│   ├── api.types.ts          # API request/response types
│   └── express.d.ts          # Express type extensions
├── schemas/
│   ├── index.ts              # Barrel export
│   ├── user.schema.ts        # User Zod schemas
│   ├── auth.schema.ts        # Auth Zod schemas
│   └── common.schema.ts      # Shared schemas (pagination, etc.)
├── domain/
│   ├── entities/
│   │   └── User.ts           # User entity class
│   ├── repositories/
│   │   ├── IUserRepository.ts    # Repository interface
│   │   └── UserRepository.ts     # PostgreSQL implementation
│   └── services/
│       ├── AuthService.ts
│       ├── TokenService.ts
│       └── UserService.ts
├── infrastructure/
│   ├── database/
│   │   ├── connection.ts     # Database connection pool
│   │   └── migrations.ts     # Migration runner
│   ├── logging/
│   │   └── logger.ts         # Pino logger setup
│   └── metrics/
│       └── prometheus.ts     # Metrics setup
├── api/
│   ├── middleware/
│   │   ├── auth.middleware.ts
│   │   ├── error.middleware.ts
│   │   ├── validation.middleware.ts
│   │   ├── rateLimit.middleware.ts
│   │   └── requestId.middleware.ts
│   ├── routes/
│   │   ├── index.ts          # Route aggregator
│   │   ├── auth.routes.ts
│   │   ├── user.routes.ts
│   │   └── health.routes.ts
│   └── controllers/
│       ├── AuthController.ts
│       ├── UserController.ts
│       └── HealthController.ts
├── utils/
│   ├── errors.ts             # Custom error classes
│   ├── password.ts           # Password hashing utilities
│   └── response.ts           # API response helpers
├── app.ts                    # Express app setup
└── server.ts                 # Entry point
```

### 1.6 Create Environment Example File

Create `.env.example`:
```bash
# Server Configuration
NODE_ENV=development
PORT=8080

# Database Configuration
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=auth_db
DATABASE_USER=auth_user
DATABASE_PASSWORD=your_secure_password_here
DATABASE_POOL_MAX=20
DATABASE_SSL=false

# JWT Configuration
JWT_SECRET=your-super-secret-jwt-key-min-32-chars
JWT_ACCESS_TOKEN_EXPIRY=15m
JWT_REFRESH_TOKEN_EXPIRY=7d

# Security Configuration
RATE_LIMIT_WINDOW_MS=900000
RATE_LIMIT_MAX_REQUESTS=100
ACCOUNT_LOCKOUT_ATTEMPTS=5
ACCOUNT_LOCKOUT_DURATION_MS=1800000
PASSWORD_MIN_LENGTH=8

# External Services (optional)
STORAGE_SERVICE_URL=http://storage-service:8080
CACHE_SERVICE_URL=http://cache-service:8080

# Metrics
METRICS_ENABLED=true
METRICS_PREFIX=auth_service_

# Logging
LOG_LEVEL=info
LOG_PRETTY=true
```

---

## Phase 2: Core Types & Interfaces

**Duration**: Day 2  
**Goal**: Define all TypeScript types and interfaces

### 2.1 User Domain Types

Create `src/types/user.types.ts`:
```typescript
export interface User {
  id: string;
  email: string;
  hashedPassword: string;
  salt: string;
  role: UserRole;
  isActive: boolean;
  failedAttempts: number;
  lockedUntil: Date | null;
  createdAt: Date;
  updatedAt: Date;
}

export type UserRole = 'user' | 'admin';

export interface SafeUser {
  id: string;
  email: string;
  role: UserRole;
  isActive: boolean;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateUserDTO {
  email: string;
  password: string;
  role?: UserRole;
}

export interface UpdateUserDTO {
  email?: string;
  password?: string;
  role?: UserRole;
  isActive?: boolean;
}

export interface UserFilters {
  role?: UserRole;
  isActive?: boolean;
}

export interface PaginationParams {
  page: number;
  pageSize: number;
}

export interface PaginatedResult<T> {
  data: T[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
}
```

### 2.2 Auth Domain Types

Create `src/types/auth.types.ts`:
```typescript
import type { SafeUser } from './user.types.js';

export interface TokenPair {
  accessToken: string;
  refreshToken: string;
  tokenType: 'bearer';
  expiresIn: number;
}

export interface TokenPayload {
  userId: string;
  email: string;
  role: string;
  tokenType: 'ACCESS_TOKEN' | 'REFRESH_TOKEN';
  jti: string;
  iat: number;
  exp: number;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  user: SafeUser;
  tokens: TokenPair;
}

export interface TokenValidationResult {
  valid: boolean;
  user?: SafeUser;
  error?: string;
}

export interface RefreshTokenRequest {
  refreshToken: string;
}

export interface AuthenticatedUser {
  id: string;
  email: string;
  role: string;
}
```

### 2.3 API Response Types

Create `src/types/api.types.ts`:
```typescript
export interface ApiResponse<T = unknown> {
  status: 'success' | 'error';
  message: string;
  data: T | null;
  requestId?: string;
  timestamp?: string;
}

export interface ApiError {
  status: 'error';
  message: string;
  code: string;
  details?: Record<string, unknown>;
  stack?: string;
}

export interface HealthStatus {
  status: 'healthy' | 'degraded' | 'unhealthy';
  service: string;
  version: string;
  timestamp: string;
  uptime: number;
  dependencies: Record<string, 'healthy' | 'unhealthy'>;
}

export interface ReadinessStatus {
  status: 'ready' | 'not ready';
  timestamp: string;
  message: string;
}

export interface LivenessStatus {
  status: 'alive';
  timestamp: string;
  uptime: number;
  memory: NodeJS.MemoryUsage;
}
```

### 2.4 Express Type Extensions

Create `src/types/express.d.ts`:
```typescript
import type { AuthenticatedUser } from './auth.types.js';

declare global {
  namespace Express {
    interface Request {
      id: string;
      user?: AuthenticatedUser;
      startTime?: number;
    }
  }
}

export {};
```

### 2.5 Barrel Export

Create `src/types/index.ts`:
```typescript
export * from './user.types.js';
export * from './auth.types.js';
export * from './api.types.js';
```

---

## Phase 3: Infrastructure Migration

**Duration**: Days 3-4  
**Goal**: Migrate config, database, logging, and error handling

### 3.1 Environment Configuration with Zod

Create `src/config/env.ts`:
```typescript
import { z } from 'zod';

const envSchema = z.object({
  // Server
  NODE_ENV: z.enum(['development', 'production', 'test']).default('development'),
  PORT: z.coerce.number().default(8080),

  // Database
  DATABASE_HOST: z.string().default('localhost'),
  DATABASE_PORT: z.coerce.number().default(5432),
  DATABASE_NAME: z.string().default('auth_db'),
  DATABASE_USER: z.string().default('auth_user'),
  DATABASE_PASSWORD: z.string().min(1),
  DATABASE_POOL_MAX: z.coerce.number().default(20),
  DATABASE_SSL: z.coerce.boolean().default(false),

  // JWT
  JWT_SECRET: z.string().min(32),
  JWT_ACCESS_TOKEN_EXPIRY: z.string().default('15m'),
  JWT_REFRESH_TOKEN_EXPIRY: z.string().default('7d'),

  // Security
  RATE_LIMIT_WINDOW_MS: z.coerce.number().default(900000),
  RATE_LIMIT_MAX_REQUESTS: z.coerce.number().default(100),
  ACCOUNT_LOCKOUT_ATTEMPTS: z.coerce.number().default(5),
  ACCOUNT_LOCKOUT_DURATION_MS: z.coerce.number().default(1800000),
  PASSWORD_MIN_LENGTH: z.coerce.number().default(8),

  // External Services
  STORAGE_SERVICE_URL: z.string().url().optional(),
  CACHE_SERVICE_URL: z.string().url().optional(),

  // Metrics & Logging
  METRICS_ENABLED: z.coerce.boolean().default(true),
  METRICS_PREFIX: z.string().default('auth_service_'),
  LOG_LEVEL: z.enum(['fatal', 'error', 'warn', 'info', 'debug', 'trace']).default('info'),
  LOG_PRETTY: z.coerce.boolean().default(false),
});

export type Env = z.infer<typeof envSchema>;

function validateEnv(): Env {
  const result = envSchema.safeParse(process.env);

  if (!result.success) {
    console.error('❌ Invalid environment variables:');
    console.error(result.error.format());
    process.exit(1);
  }

  return result.data;
}

export const env = validateEnv();
```

Create `src/config/index.ts`:
```typescript
import { env } from './env.js';

export const config = {
  server: {
    port: env.PORT,
    nodeEnv: env.NODE_ENV,
    isDevelopment: env.NODE_ENV === 'development',
    isProduction: env.NODE_ENV === 'production',
    isTest: env.NODE_ENV === 'test',
  },

  database: {
    host: env.DATABASE_HOST,
    port: env.DATABASE_PORT,
    database: env.DATABASE_NAME,
    user: env.DATABASE_USER,
    password: env.DATABASE_PASSWORD,
    max: env.DATABASE_POOL_MAX,
    ssl: env.DATABASE_SSL,
  },

  jwt: {
    secret: env.JWT_SECRET,
    accessTokenExpiry: env.JWT_ACCESS_TOKEN_EXPIRY,
    refreshTokenExpiry: env.JWT_REFRESH_TOKEN_EXPIRY,
  },

  security: {
    rateLimitWindowMs: env.RATE_LIMIT_WINDOW_MS,
    rateLimitMaxRequests: env.RATE_LIMIT_MAX_REQUESTS,
    accountLockoutAttempts: env.ACCOUNT_LOCKOUT_ATTEMPTS,
    accountLockoutDurationMs: env.ACCOUNT_LOCKOUT_DURATION_MS,
    passwordMinLength: env.PASSWORD_MIN_LENGTH,
  },

  services: {
    storageUrl: env.STORAGE_SERVICE_URL,
    cacheUrl: env.CACHE_SERVICE_URL,
  },

  metrics: {
    enabled: env.METRICS_ENABLED,
    prefix: env.METRICS_PREFIX,
  },

  logging: {
    level: env.LOG_LEVEL,
    pretty: env.LOG_PRETTY,
  },
} as const;

export type Config = typeof config;
export { env };
```

### 3.2 Logger Setup with Pino

Install pino:
```bash
npm install pino pino-pretty
npm install --save-dev @types/pino
```

Create `src/infrastructure/logging/logger.ts`:
```typescript
import pino from 'pino';
import { config } from '../../config/index.js';

const transport = config.logging.pretty
  ? {
      target: 'pino-pretty',
      options: {
        colorize: true,
        translateTime: 'SYS:standard',
        ignore: 'pid,hostname',
      },
    }
  : undefined;

export const logger = pino({
  level: config.logging.level,
  transport,
  base: {
    service: 'auth-service',
    version: process.env.npm_package_version ?? '2.0.0',
    env: config.server.nodeEnv,
  },
  timestamp: pino.stdTimeFunctions.isoTime,
  formatters: {
    level: (label) => ({ level: label }),
  },
});

export const createChildLogger = (bindings: Record<string, unknown>) => {
  return logger.child(bindings);
};

export type Logger = typeof logger;
```

### 3.3 Custom Error Classes

Create `src/utils/errors.ts`:
```typescript
export class AppError extends Error {
  constructor(
    message: string,
    public readonly statusCode: number = 500,
    public readonly code: string = 'INTERNAL_ERROR',
    public readonly isOperational: boolean = true,
    public readonly details?: Record<string, unknown>
  ) {
    super(message);
    this.name = this.constructor.name;
    Error.captureStackTrace(this, this.constructor);
  }
}

export class ValidationError extends AppError {
  constructor(message: string, details?: Record<string, unknown>) {
    super(message, 400, 'VALIDATION_ERROR', true, details);
  }
}

export class UnauthorizedError extends AppError {
  constructor(message: string = 'Unauthorized') {
    super(message, 401, 'UNAUTHORIZED', true);
  }
}

export class ForbiddenError extends AppError {
  constructor(message: string = 'Access denied') {
    super(message, 403, 'FORBIDDEN', true);
  }
}

export class NotFoundError extends AppError {
  constructor(resource: string = 'Resource') {
    super(`${resource} not found`, 404, 'NOT_FOUND', true);
  }
}

export class ConflictError extends AppError {
  constructor(message: string) {
    super(message, 409, 'CONFLICT', true);
  }
}

export class TooManyRequestsError extends AppError {
  constructor(message: string = 'Too many requests') {
    super(message, 429, 'TOO_MANY_REQUESTS', true);
  }
}

export class AccountLockedError extends AppError {
  constructor(lockedUntil?: Date) {
    const message = lockedUntil
      ? `Account is locked until ${lockedUntil.toISOString()}`
      : 'Account is temporarily locked';
    super(message, 423, 'ACCOUNT_LOCKED', true, { lockedUntil });
  }
}

export class DatabaseError extends AppError {
  constructor(message: string, originalError?: Error) {
    super(message, 500, 'DATABASE_ERROR', false, {
      originalError: originalError?.message,
    });
  }
}
```

### 3.4 Database Connection

Create `src/infrastructure/database/connection.ts`:
```typescript
import pg from 'pg';
import { config } from '../../config/index.js';
import { logger } from '../logging/logger.js';
import { DatabaseError } from '../../utils/errors.js';

const { Pool } = pg;

export interface DatabaseClient {
  query<T extends pg.QueryResultRow = pg.QueryResultRow>(
    text: string,
    params?: unknown[]
  ): Promise<pg.QueryResult<T>>;
  release(): void;
}

class Database {
  private pool: pg.Pool;
  private isConnected = false;

  constructor() {
    this.pool = new Pool({
      host: config.database.host,
      port: config.database.port,
      database: config.database.database,
      user: config.database.user,
      password: config.database.password,
      max: config.database.max,
      idleTimeoutMillis: 30000,
      connectionTimeoutMillis: 10000,
      ssl: config.database.ssl ? { rejectUnauthorized: false } : false,
    });

    this.setupEventHandlers();
  }

  private setupEventHandlers(): void {
    this.pool.on('connect', () => {
      this.isConnected = true;
      logger.debug('New database connection established');
    });

    this.pool.on('error', (err) => {
      logger.error({ err }, 'Unexpected database pool error');
    });

    this.pool.on('remove', () => {
      logger.debug('Database connection removed from pool');
    });
  }

  async query<T extends pg.QueryResultRow = pg.QueryResultRow>(
    text: string,
    params?: unknown[]
  ): Promise<pg.QueryResult<T>> {
    const start = Date.now();
    try {
      const result = await this.pool.query<T>(text, params);
      const duration = Date.now() - start;
      logger.debug({ query: text, duration, rows: result.rowCount }, 'Query executed');
      return result;
    } catch (error) {
      logger.error({ err: error, query: text }, 'Query failed');
      throw new DatabaseError('Database query failed', error as Error);
    }
  }

  async getClient(): Promise<DatabaseClient> {
    return this.pool.connect();
  }

  async transaction<T>(
    callback: (client: DatabaseClient) => Promise<T>
  ): Promise<T> {
    const client = await this.getClient();
    try {
      await client.query('BEGIN');
      const result = await callback(client);
      await client.query('COMMIT');
      return result;
    } catch (error) {
      await client.query('ROLLBACK');
      throw error;
    } finally {
      client.release();
    }
  }

  async healthCheck(): Promise<boolean> {
    try {
      await this.pool.query('SELECT 1');
      return true;
    } catch {
      return false;
    }
  }

  async close(): Promise<void> {
    await this.pool.end();
    this.isConnected = false;
    logger.info('Database pool closed');
  }

  get connected(): boolean {
    return this.isConnected;
  }
}

export const db = new Database();
```

---

## Phase 4: Business Logic Migration

**Duration**: Days 5-7  
**Goal**: Migrate domain entities, repositories, and services

### 4.1 User Entity

Create `src/domain/entities/User.ts`:
```typescript
import type { User, SafeUser, UserRole } from '../../types/index.js';

export class UserEntity implements User {
  constructor(
    public readonly id: string,
    public readonly email: string,
    public readonly hashedPassword: string,
    public readonly salt: string,
    public readonly role: UserRole,
    public readonly isActive: boolean,
    public readonly failedAttempts: number,
    public readonly lockedUntil: Date | null,
    public readonly createdAt: Date,
    public readonly updatedAt: Date
  ) {}

  isLocked(): boolean {
    if (!this.lockedUntil) return false;
    return new Date() < this.lockedUntil;
  }

  canLogin(): boolean {
    return this.isActive && !this.isLocked();
  }

  toSafeUser(): SafeUser {
    return {
      id: this.id,
      email: this.email,
      role: this.role,
      isActive: this.isActive,
      createdAt: this.createdAt,
      updatedAt: this.updatedAt,
    };
  }

  static fromRow(row: UserRow): UserEntity {
    return new UserEntity(
      row.id,
      row.email,
      row.hashed_password,
      row.salt,
      row.role as UserRole,
      row.is_active,
      row.failed_attempts,
      row.locked_until,
      row.created_at,
      row.updated_at
    );
  }
}

export interface UserRow {
  id: string;
  email: string;
  hashed_password: string;
  salt: string;
  role: string;
  is_active: boolean;
  failed_attempts: number;
  locked_until: Date | null;
  created_at: Date;
  updated_at: Date;
}
```

### 4.2 Repository Interface

Create `src/domain/repositories/IUserRepository.ts`:
```typescript
import type { CreateUserDTO, UpdateUserDTO, PaginationParams, PaginatedResult } from '../../types/index.js';
import type { UserEntity } from '../entities/User.js';

export interface IUserRepository {
  create(data: CreateUserDTO): Promise<UserEntity>;
  findById(id: string): Promise<UserEntity | null>;
  findByEmail(email: string): Promise<UserEntity | null>;
  update(id: string, data: UpdateUserDTO): Promise<UserEntity | null>;
  delete(id: string): Promise<boolean>;
  list(params: PaginationParams): Promise<PaginatedResult<UserEntity>>;
  recordLoginAttempt(id: string, success: boolean): Promise<UserEntity | null>;
  validatePassword(user: UserEntity, password: string): Promise<boolean>;
}
```

### 4.3 User Repository Implementation

Create `src/domain/repositories/UserRepository.ts`:
```typescript
import bcrypt from 'bcrypt';
import crypto from 'node:crypto';
import { v4 as uuidv4 } from 'uuid';
import type { IUserRepository } from './IUserRepository.js';
import type { CreateUserDTO, UpdateUserDTO, PaginationParams, PaginatedResult } from '../../types/index.js';
import { UserEntity, type UserRow } from '../entities/User.js';
import { db } from '../../infrastructure/database/connection.js';
import { config } from '../../config/index.js';
import { logger } from '../../infrastructure/logging/logger.js';

export class UserRepository implements IUserRepository {
  private readonly SALT_ROUNDS = 12;
  private readonly log = logger.child({ repository: 'UserRepository' });

  async create(data: CreateUserDTO): Promise<UserEntity> {
    const id = uuidv4();
    const salt = crypto.randomBytes(32).toString('hex');
    const hashedPassword = await bcrypt.hash(data.password + salt, this.SALT_ROUNDS);
    const now = new Date();

    const query = `
      INSERT INTO users (id, email, hashed_password, salt, role, is_active, created_at, updated_at)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
      RETURNING *
    `;

    const result = await db.query<UserRow>(query, [
      id,
      data.email.toLowerCase().trim(),
      hashedPassword,
      salt,
      data.role ?? 'user',
      true,
      now,
      now,
    ]);

    this.log.info({ userId: id, email: data.email }, 'User created');
    return UserEntity.fromRow(result.rows[0]!);
  }

  async findById(id: string): Promise<UserEntity | null> {
    const result = await db.query<UserRow>('SELECT * FROM users WHERE id = $1', [id]);
    return result.rows[0] ? UserEntity.fromRow(result.rows[0]) : null;
  }

  async findByEmail(email: string): Promise<UserEntity | null> {
    const result = await db.query<UserRow>('SELECT * FROM users WHERE email = $1', [
      email.toLowerCase().trim(),
    ]);
    return result.rows[0] ? UserEntity.fromRow(result.rows[0]) : null;
  }

  async update(id: string, data: UpdateUserDTO): Promise<UserEntity | null> {
    const updates: string[] = [];
    const values: unknown[] = [];
    let paramCount = 1;

    if (data.email !== undefined) {
      updates.push(`email = $${paramCount++}`);
      values.push(data.email.toLowerCase().trim());
    }

    if (data.role !== undefined) {
      updates.push(`role = $${paramCount++}`);
      values.push(data.role);
    }

    if (data.isActive !== undefined) {
      updates.push(`is_active = $${paramCount++}`);
      values.push(data.isActive);
    }

    if (data.password !== undefined) {
      const salt = crypto.randomBytes(32).toString('hex');
      const hashedPassword = await bcrypt.hash(data.password + salt, this.SALT_ROUNDS);
      updates.push(`hashed_password = $${paramCount++}`);
      values.push(hashedPassword);
      updates.push(`salt = $${paramCount++}`);
      values.push(salt);
    }

    if (updates.length === 0) {
      return this.findById(id);
    }

    updates.push(`updated_at = $${paramCount++}`);
    values.push(new Date());
    values.push(id);

    const query = `
      UPDATE users SET ${updates.join(', ')}
      WHERE id = $${paramCount}
      RETURNING *
    `;

    const result = await db.query<UserRow>(query, values);
    if (!result.rows[0]) return null;

    this.log.info({ userId: id }, 'User updated');
    return UserEntity.fromRow(result.rows[0]);
  }

  async delete(id: string): Promise<boolean> {
    const result = await db.query('DELETE FROM users WHERE id = $1 RETURNING id', [id]);
    const deleted = result.rowCount !== null && result.rowCount > 0;
    if (deleted) {
      this.log.info({ userId: id }, 'User deleted');
    }
    return deleted;
  }

  async list(params: PaginationParams): Promise<PaginatedResult<UserEntity>> {
    const { page, pageSize } = params;
    const offset = (page - 1) * pageSize;

    const [dataResult, countResult] = await Promise.all([
      db.query<UserRow>(
        'SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2',
        [pageSize, offset]
      ),
      db.query<{ count: string }>('SELECT COUNT(*) as count FROM users'),
    ]);

    const total = parseInt(countResult.rows[0]?.count ?? '0', 10);

    return {
      data: dataResult.rows.map((row) => UserEntity.fromRow(row)),
      pagination: {
        page,
        pageSize,
        total,
        totalPages: Math.ceil(total / pageSize),
      },
    };
  }

  async recordLoginAttempt(id: string, success: boolean): Promise<UserEntity | null> {
    const lockoutDuration = config.security.accountLockoutDurationMs;
    const maxAttempts = config.security.accountLockoutAttempts;

    const query = `
      UPDATE users SET
        failed_attempts = CASE 
          WHEN $2 = true THEN 0 
          ELSE failed_attempts + 1 
        END,
        locked_until = CASE 
          WHEN $2 = false AND failed_attempts >= $3 
            THEN NOW() + ($4 || ' milliseconds')::interval
          WHEN $2 = true THEN NULL
          ELSE locked_until 
        END,
        updated_at = NOW()
      WHERE id = $1
      RETURNING *
    `;

    const result = await db.query<UserRow>(query, [id, success, maxAttempts - 1, lockoutDuration]);
    return result.rows[0] ? UserEntity.fromRow(result.rows[0]) : null;
  }

  async validatePassword(user: UserEntity, password: string): Promise<boolean> {
    return bcrypt.compare(password + user.salt, user.hashedPassword);
  }
}

// Export singleton for dependency injection container
export const userRepository = new UserRepository();
```

### 4.4 Token Service

Create `src/domain/services/TokenService.ts`:
```typescript
import jwt from 'jsonwebtoken';
import { v4 as uuidv4 } from 'uuid';
import type { TokenPair, TokenPayload } from '../../types/index.js';
import type { UserEntity } from '../entities/User.js';
import { config } from '../../config/index.js';
import { UnauthorizedError } from '../../utils/errors.js';
import { logger } from '../../infrastructure/logging/logger.js';

export class TokenService {
  private readonly log = logger.child({ service: 'TokenService' });

  generateTokens(user: UserEntity): TokenPair {
    const jti = uuidv4();

    const basePayload = {
      userId: user.id,
      email: user.email,
      role: user.role,
      jti,
    };

    const accessToken = jwt.sign(
      { ...basePayload, tokenType: 'ACCESS_TOKEN' as const },
      config.jwt.secret,
      { expiresIn: config.jwt.accessTokenExpiry, algorithm: 'HS256' }
    );

    const refreshToken = jwt.sign(
      { ...basePayload, tokenType: 'REFRESH_TOKEN' as const },
      config.jwt.secret,
      { expiresIn: config.jwt.refreshTokenExpiry, algorithm: 'HS256' }
    );

    this.log.debug({ userId: user.id, jti }, 'Tokens generated');

    return {
      accessToken,
      refreshToken,
      tokenType: 'bearer',
      expiresIn: this.parseExpiryToSeconds(config.jwt.accessTokenExpiry),
    };
  }

  verifyAccessToken(token: string): TokenPayload {
    try {
      const decoded = jwt.verify(token, config.jwt.secret, {
        algorithms: ['HS256'],
      }) as TokenPayload;

      if (decoded.tokenType !== 'ACCESS_TOKEN') {
        throw new UnauthorizedError('Invalid token type');
      }

      return decoded;
    } catch (error) {
      if (error instanceof jwt.TokenExpiredError) {
        throw new UnauthorizedError('Token expired');
      }
      if (error instanceof jwt.JsonWebTokenError) {
        throw new UnauthorizedError('Invalid token');
      }
      throw error;
    }
  }

  verifyRefreshToken(token: string): TokenPayload {
    try {
      const decoded = jwt.verify(token, config.jwt.secret, {
        algorithms: ['HS256'],
      }) as TokenPayload;

      if (decoded.tokenType !== 'REFRESH_TOKEN') {
        throw new UnauthorizedError('Invalid refresh token');
      }

      return decoded;
    } catch (error) {
      if (error instanceof jwt.TokenExpiredError) {
        throw new UnauthorizedError('Refresh token expired');
      }
      if (error instanceof jwt.JsonWebTokenError) {
        throw new UnauthorizedError('Invalid refresh token');
      }
      throw error;
    }
  }

  decodeToken(token: string): TokenPayload | null {
    return jwt.decode(token) as TokenPayload | null;
  }

  private parseExpiryToSeconds(expiry: string): number {
    const match = expiry.match(/^(\d+)([smhd])$/);
    if (!match) return 3600; // Default 1 hour

    const [, value, unit] = match;
    const num = parseInt(value!, 10);

    switch (unit) {
      case 's': return num;
      case 'm': return num * 60;
      case 'h': return num * 3600;
      case 'd': return num * 86400;
      default: return 3600;
    }
  }
}

export const tokenService = new TokenService();
```

### 4.5 Auth Service

Create `src/domain/services/AuthService.ts`:
```typescript
import type { LoginRequest, LoginResponse, TokenValidationResult, TokenPair } from '../../types/index.js';
import type { IUserRepository } from '../repositories/IUserRepository.js';
import { TokenService } from './TokenService.js';
import { UnauthorizedError, AccountLockedError, NotFoundError } from '../../utils/errors.js';
import { logger } from '../../infrastructure/logging/logger.js';

export class AuthService {
  private readonly log = logger.child({ service: 'AuthService' });

  constructor(
    private readonly userRepository: IUserRepository,
    private readonly tokenService: TokenService
  ) {}

  async login(request: LoginRequest, clientInfo?: { ip?: string; userAgent?: string }): Promise<LoginResponse> {
    const { email, password } = request;
    this.log.info({ email }, 'Login attempt');

    // Find user
    const user = await this.userRepository.findByEmail(email);
    if (!user) {
      this.log.warn({ email, reason: 'USER_NOT_FOUND' }, 'Login failed');
      throw new UnauthorizedError('Invalid credentials');
    }

    // Check if account is locked
    if (user.isLocked()) {
      this.log.warn({ userId: user.id, lockedUntil: user.lockedUntil }, 'Login attempt on locked account');
      throw new AccountLockedError(user.lockedUntil ?? undefined);
    }

    // Check if account is active
    if (!user.isActive) {
      this.log.warn({ userId: user.id }, 'Login attempt on inactive account');
      throw new UnauthorizedError('Account is inactive');
    }

    // Validate password
    const isValid = await this.userRepository.validatePassword(user, password);
    if (!isValid) {
      await this.userRepository.recordLoginAttempt(user.id, false);
      this.log.warn({ userId: user.id, reason: 'INVALID_PASSWORD' }, 'Login failed');
      throw new UnauthorizedError('Invalid credentials');
    }

    // Record successful login
    await this.userRepository.recordLoginAttempt(user.id, true);

    // Generate tokens
    const tokens = this.tokenService.generateTokens(user);

    this.log.info({ userId: user.id }, 'Login successful');

    return {
      user: user.toSafeUser(),
      tokens,
    };
  }

  async validateToken(token: string): Promise<TokenValidationResult> {
    try {
      const decoded = this.tokenService.verifyAccessToken(token);
      const user = await this.userRepository.findById(decoded.userId);

      if (!user || !user.isActive) {
        return { valid: false, error: 'User not found or inactive' };
      }

      return { valid: true, user: user.toSafeUser() };
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Token validation failed';
      return { valid: false, error: message };
    }
  }

  async refreshTokens(refreshToken: string): Promise<TokenPair> {
    const decoded = this.tokenService.verifyRefreshToken(refreshToken);
    const user = await this.userRepository.findById(decoded.userId);

    if (!user || !user.isActive) {
      throw new UnauthorizedError('User not found or inactive');
    }

    this.log.info({ userId: user.id }, 'Token refreshed');
    return this.tokenService.generateTokens(user);
  }

  async logout(token: string): Promise<void> {
    // In a production system, you'd want to:
    // 1. Add the token to a blacklist (Redis)
    // 2. Or track active sessions and invalidate
    const decoded = this.tokenService.decodeToken(token);
    if (decoded) {
      this.log.info({ userId: decoded.userId, jti: decoded.jti }, 'User logged out');
    }
  }
}
```

### 4.6 User Service

Create `src/domain/services/UserService.ts`:
```typescript
import type { CreateUserDTO, UpdateUserDTO, PaginationParams, SafeUser, PaginatedResult } from '../../types/index.js';
import type { IUserRepository } from '../repositories/IUserRepository.js';
import { NotFoundError, ConflictError, ValidationError } from '../../utils/errors.js';
import { config } from '../../config/index.js';
import { logger } from '../../infrastructure/logging/logger.js';

export class UserService {
  private readonly log = logger.child({ service: 'UserService' });

  constructor(private readonly userRepository: IUserRepository) {}

  async createUser(data: CreateUserDTO): Promise<SafeUser> {
    // Check password length
    if (data.password.length < config.security.passwordMinLength) {
      throw new ValidationError(
        `Password must be at least ${config.security.passwordMinLength} characters`
      );
    }

    // Check if email already exists
    const existing = await this.userRepository.findByEmail(data.email);
    if (existing) {
      throw new ConflictError('User with this email already exists');
    }

    const user = await this.userRepository.create(data);
    this.log.info({ userId: user.id, email: user.email }, 'User created');
    return user.toSafeUser();
  }

  async getUserById(id: string): Promise<SafeUser> {
    const user = await this.userRepository.findById(id);
    if (!user) {
      throw new NotFoundError('User');
    }
    return user.toSafeUser();
  }

  async getUserByEmail(email: string): Promise<SafeUser> {
    const user = await this.userRepository.findByEmail(email);
    if (!user) {
      throw new NotFoundError('User');
    }
    return user.toSafeUser();
  }

  async updateUser(id: string, data: UpdateUserDTO): Promise<SafeUser> {
    // Validate password if being updated
    if (data.password && data.password.length < config.security.passwordMinLength) {
      throw new ValidationError(
        `Password must be at least ${config.security.passwordMinLength} characters`
      );
    }

    // Check email uniqueness if being updated
    if (data.email) {
      const existing = await this.userRepository.findByEmail(data.email);
      if (existing && existing.id !== id) {
        throw new ConflictError('User with this email already exists');
      }
    }

    const user = await this.userRepository.update(id, data);
    if (!user) {
      throw new NotFoundError('User');
    }

    this.log.info({ userId: id }, 'User updated');
    return user.toSafeUser();
  }

  async deleteUser(id: string): Promise<void> {
    const deleted = await this.userRepository.delete(id);
    if (!deleted) {
      throw new NotFoundError('User');
    }
    this.log.info({ userId: id }, 'User deleted');
  }

  async listUsers(params: PaginationParams): Promise<PaginatedResult<SafeUser>> {
    const result = await this.userRepository.list(params);
    return {
      data: result.data.map((user) => user.toSafeUser()),
      pagination: result.pagination,
    };
  }

  async deactivateUser(id: string): Promise<SafeUser> {
    return this.updateUser(id, { isActive: false });
  }

  async activateUser(id: string): Promise<SafeUser> {
    return this.updateUser(id, { isActive: true });
  }
}
```

---

## Phase 5: Validation & Error Handling

**Duration**: Day 8  
**Goal**: Add Zod validation schemas and error handling middleware

### 5.1 Install Zod

```bash
npm install zod
```

### 5.2 Auth Schemas

Create `src/schemas/auth.schema.ts`:
```typescript
import { z } from 'zod';

export const loginSchema = z.object({
  body: z.object({
    email: z.string().email('Invalid email format'),
    password: z.string().min(1, 'Password is required'),
    // Support legacy field name
    user_id: z.string().email('Invalid email format').optional(),
  }).transform((data) => ({
    email: data.user_id ?? data.email,
    password: data.password,
  })),
});

export const refreshTokenSchema = z.object({
  body: z.object({
    refresh_token: z.string().min(1, 'Refresh token is required'),
  }),
});

export const validateTokenSchema = z.object({
  body: z.object({
    token: z.string().optional(),
  }),
  headers: z.object({
    authorization: z.string().optional(),
  }),
}).transform((data) => ({
  token: data.body.token ?? data.headers.authorization?.replace('Bearer ', ''),
}));

export type LoginInput = z.infer<typeof loginSchema>['body'];
export type RefreshTokenInput = z.infer<typeof refreshTokenSchema>['body'];
```

### 5.3 User Schemas

Create `src/schemas/user.schema.ts`:
```typescript
import { z } from 'zod';

const userRoleSchema = z.enum(['user', 'admin']);

export const createUserSchema = z.object({
  body: z.object({
    email: z.string().email('Invalid email format'),
    password: z.string().min(8, 'Password must be at least 8 characters'),
    role: userRoleSchema.optional().default('user'),
  }),
});

export const updateUserSchema = z.object({
  params: z.object({
    id: z.string().uuid('Invalid user ID'),
  }),
  body: z.object({
    email: z.string().email('Invalid email format').optional(),
    password: z.string().min(8, 'Password must be at least 8 characters').optional(),
    role: userRoleSchema.optional(),
    isActive: z.boolean().optional(),
  }),
});

export const getUserSchema = z.object({
  params: z.object({
    id: z.string().uuid('Invalid user ID'),
  }),
});

export const getUserByEmailSchema = z.object({
  params: z.object({
    email: z.string().email('Invalid email format'),
  }),
});

export const listUsersSchema = z.object({
  query: z.object({
    page: z.coerce.number().int().positive().optional().default(1),
    pageSize: z.coerce.number().int().min(1).max(100).optional().default(10),
  }),
});

export const changeRoleSchema = z.object({
  params: z.object({
    id: z.string().uuid('Invalid user ID'),
  }),
  body: z.object({
    role: userRoleSchema,
  }),
});

export type CreateUserInput = z.infer<typeof createUserSchema>['body'];
export type UpdateUserInput = z.infer<typeof updateUserSchema>['body'];
export type ListUsersInput = z.infer<typeof listUsersSchema>['query'];
```

### 5.4 Validation Middleware

Create `src/api/middleware/validation.middleware.ts`:
```typescript
import type { Request, Response, NextFunction } from 'express';
import type { AnyZodObject, ZodError } from 'zod';
import { ValidationError } from '../../utils/errors.js';

export const validate = (schema: AnyZodObject) => {
  return async (req: Request, _res: Response, next: NextFunction): Promise<void> => {
    try {
      const result = await schema.parseAsync({
        body: req.body,
        query: req.query,
        params: req.params,
        headers: req.headers,
      });

      // Merge parsed data back into request
      if (result.body) req.body = result.body;
      if (result.query) req.query = result.query;
      if (result.params) req.params = result.params;

      next();
    } catch (error) {
      const zodError = error as ZodError;
      const details = zodError.errors.reduce<Record<string, string>>((acc, err) => {
        const path = err.path.join('.');
        acc[path] = err.message;
        return acc;
      }, {});

      next(new ValidationError('Validation failed', details));
    }
  };
};
```

### 5.5 Error Handling Middleware

Create `src/api/middleware/error.middleware.ts`:
```typescript
import type { Request, Response, NextFunction, ErrorRequestHandler } from 'express';
import { AppError } from '../../utils/errors.js';
import { config } from '../../config/index.js';
import { logger } from '../../infrastructure/logging/logger.js';
import type { ApiError } from '../../types/index.js';

export const errorHandler: ErrorRequestHandler = (
  error: Error,
  req: Request,
  res: Response,
  _next: NextFunction
): void => {
  const requestId = req.id;

  // Log the error
  logger.error(
    {
      err: error,
      requestId,
      path: req.path,
      method: req.method,
    },
    'Request error'
  );

  // Handle operational errors
  if (error instanceof AppError) {
    const response: ApiError = {
      status: 'error',
      message: error.message,
      code: error.code,
      details: error.details,
    };

    if (config.server.isDevelopment) {
      response.stack = error.stack;
    }

    res.status(error.statusCode).json(response);
    return;
  }

  // Handle unknown errors
  const response: ApiError = {
    status: 'error',
    message: config.server.isProduction
      ? 'An internal server error occurred'
      : error.message,
    code: 'INTERNAL_ERROR',
  };

  if (config.server.isDevelopment) {
    response.stack = error.stack;
  }

  res.status(500).json(response);
};

export const notFoundHandler = (req: Request, res: Response): void => {
  res.status(404).json({
    status: 'error',
    message: `Route ${req.method} ${req.path} not found`,
    code: 'NOT_FOUND',
  });
};
```

---

## Phase 6: Testing Setup

**Duration**: Days 9-11  
**Goal**: Set up Vitest with unit and integration tests

### 6.1 Install Testing Dependencies

```bash
npm install --save-dev \
  vitest@^2 \
  @vitest/coverage-v8 \
  supertest@^7 \
  @types/supertest
```

### 6.2 Vitest Configuration

Create `vitest.config.ts`:
```typescript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
    include: ['src/**/*.{test,spec}.ts'],
    exclude: ['node_modules', 'dist'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: ['src/**/*.ts'],
      exclude: [
        'src/**/*.{test,spec}.ts',
        'src/**/__tests__/**',
        'src/types/**',
        'src/server.ts',
      ],
      thresholds: {
        lines: 70,
        functions: 70,
        branches: 60,
        statements: 70,
      },
    },
    setupFiles: ['./src/__tests__/setup.ts'],
    testTimeout: 30000,
    hookTimeout: 30000,
  },
});
```

### 6.3 Test Setup File

Create `src/__tests__/setup.ts`:
```typescript
import { beforeAll, afterAll, beforeEach, vi } from 'vitest';

// Mock environment variables for testing
beforeAll(() => {
  process.env.NODE_ENV = 'test';
  process.env.JWT_SECRET = 'test-secret-key-at-least-32-characters-long';
  process.env.DATABASE_PASSWORD = 'test-password';
  process.env.LOG_LEVEL = 'silent';
});

// Reset mocks between tests
beforeEach(() => {
  vi.clearAllMocks();
});

afterAll(() => {
  vi.restoreAllMocks();
});
```

### 6.4 Test Factory Helpers

Create `src/__tests__/factories/user.factory.ts`:
```typescript
import { v4 as uuidv4 } from 'uuid';
import type { User, CreateUserDTO, SafeUser } from '../../types/index.js';
import { UserEntity } from '../../domain/entities/User.js';

export const createMockUser = (overrides: Partial<User> = {}): UserEntity => {
  const defaults: User = {
    id: uuidv4(),
    email: `test-${Date.now()}@example.com`,
    hashedPassword: 'hashed_password',
    salt: 'salt',
    role: 'user',
    isActive: true,
    failedAttempts: 0,
    lockedUntil: null,
    createdAt: new Date(),
    updatedAt: new Date(),
  };

  return new UserEntity(
    overrides.id ?? defaults.id,
    overrides.email ?? defaults.email,
    overrides.hashedPassword ?? defaults.hashedPassword,
    overrides.salt ?? defaults.salt,
    overrides.role ?? defaults.role,
    overrides.isActive ?? defaults.isActive,
    overrides.failedAttempts ?? defaults.failedAttempts,
    overrides.lockedUntil ?? defaults.lockedUntil,
    overrides.createdAt ?? defaults.createdAt,
    overrides.updatedAt ?? defaults.updatedAt
  );
};

export const createMockCreateUserDTO = (overrides: Partial<CreateUserDTO> = {}): CreateUserDTO => ({
  email: `test-${Date.now()}@example.com`,
  password: 'securePassword123',
  role: 'user',
  ...overrides,
});

export const createMockSafeUser = (overrides: Partial<SafeUser> = {}): SafeUser => ({
  id: uuidv4(),
  email: `test-${Date.now()}@example.com`,
  role: 'user',
  isActive: true,
  createdAt: new Date(),
  updatedAt: new Date(),
  ...overrides,
});
```

### 6.5 Mock Repository

Create `src/__tests__/mocks/userRepository.mock.ts`:
```typescript
import { vi } from 'vitest';
import type { IUserRepository } from '../../domain/repositories/IUserRepository.js';

export const createMockUserRepository = (): IUserRepository => ({
  create: vi.fn(),
  findById: vi.fn(),
  findByEmail: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  list: vi.fn(),
  recordLoginAttempt: vi.fn(),
  validatePassword: vi.fn(),
});
```

### 6.6 Unit Test Example: AuthService

Create `src/domain/services/__tests__/AuthService.test.ts`:
```typescript
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { AuthService } from '../AuthService.js';
import { TokenService } from '../TokenService.js';
import { createMockUserRepository } from '../../../__tests__/mocks/userRepository.mock.js';
import { createMockUser } from '../../../__tests__/factories/user.factory.js';
import { UnauthorizedError, AccountLockedError } from '../../../utils/errors.js';

describe('AuthService', () => {
  let authService: AuthService;
  let mockUserRepository: ReturnType<typeof createMockUserRepository>;
  let tokenService: TokenService;

  beforeEach(() => {
    mockUserRepository = createMockUserRepository();
    tokenService = new TokenService();
    authService = new AuthService(mockUserRepository, tokenService);
  });

  describe('login', () => {
    it('should successfully login with valid credentials', async () => {
      const mockUser = createMockUser({ email: 'test@example.com' });
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);
      mockUserRepository.validatePassword.mockResolvedValue(true);
      mockUserRepository.recordLoginAttempt.mockResolvedValue(mockUser);

      const result = await authService.login({
        email: 'test@example.com',
        password: 'password123',
      });

      expect(result.user.email).toBe('test@example.com');
      expect(result.tokens.accessToken).toBeDefined();
      expect(result.tokens.refreshToken).toBeDefined();
      expect(mockUserRepository.recordLoginAttempt).toHaveBeenCalledWith(mockUser.id, true);
    });

    it('should throw UnauthorizedError for non-existent user', async () => {
      mockUserRepository.findByEmail.mockResolvedValue(null);

      await expect(
        authService.login({ email: 'notfound@example.com', password: 'password' })
      ).rejects.toThrow(UnauthorizedError);
    });

    it('should throw UnauthorizedError for invalid password', async () => {
      const mockUser = createMockUser();
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);
      mockUserRepository.validatePassword.mockResolvedValue(false);

      await expect(
        authService.login({ email: mockUser.email, password: 'wrongpassword' })
      ).rejects.toThrow(UnauthorizedError);

      expect(mockUserRepository.recordLoginAttempt).toHaveBeenCalledWith(mockUser.id, false);
    });

    it('should throw AccountLockedError for locked account', async () => {
      const lockedUntil = new Date(Date.now() + 3600000); // 1 hour from now
      const mockUser = createMockUser({ lockedUntil });
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);

      await expect(
        authService.login({ email: mockUser.email, password: 'password' })
      ).rejects.toThrow(AccountLockedError);
    });

    it('should throw UnauthorizedError for inactive account', async () => {
      const mockUser = createMockUser({ isActive: false });
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);

      await expect(
        authService.login({ email: mockUser.email, password: 'password' })
      ).rejects.toThrow(UnauthorizedError);
    });
  });

  describe('validateToken', () => {
    it('should return valid result for valid token', async () => {
      const mockUser = createMockUser();
      mockUserRepository.findById.mockResolvedValue(mockUser);
      const tokens = tokenService.generateTokens(mockUser);

      const result = await authService.validateToken(tokens.accessToken);

      expect(result.valid).toBe(true);
      expect(result.user?.id).toBe(mockUser.id);
    });

    it('should return invalid result for inactive user', async () => {
      const mockUser = createMockUser({ isActive: false });
      mockUserRepository.findById.mockResolvedValue(mockUser);
      const tokens = tokenService.generateTokens(mockUser);

      const result = await authService.validateToken(tokens.accessToken);

      expect(result.valid).toBe(false);
    });
  });
});
```

### 6.7 Integration Test Example

Create `src/api/routes/__tests__/auth.routes.test.ts`:
```typescript
import { describe, it, expect, beforeAll, afterAll, vi } from 'vitest';
import request from 'supertest';
import { createApp } from '../../../app.js';
import type { Express } from 'express';

// Mock the database
vi.mock('../../../infrastructure/database/connection.js', () => ({
  db: {
    query: vi.fn(),
    healthCheck: vi.fn().mockResolvedValue(true),
    transaction: vi.fn(),
  },
}));

describe('Auth Routes', () => {
  let app: Express;

  beforeAll(() => {
    app = createApp();
  });

  describe('POST /v1/auth/login', () => {
    it('should return 400 for missing credentials', async () => {
      const response = await request(app)
        .post('/v1/auth/login')
        .send({});

      expect(response.status).toBe(400);
      expect(response.body.status).toBe('error');
    });

    it('should return 400 for invalid email format', async () => {
      const response = await request(app)
        .post('/v1/auth/login')
        .send({ email: 'invalid-email', password: 'password123' });

      expect(response.status).toBe(400);
    });
  });

  describe('POST /v1/auth/token/validate', () => {
    it('should return 400 when no token provided', async () => {
      const response = await request(app)
        .post('/v1/auth/token/validate')
        .send({});

      expect(response.status).toBe(400);
    });
  });

  describe('GET /health', () => {
    it('should return health status', async () => {
      const response = await request(app).get('/health');

      expect(response.status).toBe(200);
      expect(response.body.status).toBe('healthy');
    });
  });
});
```

---

## Phase 7: Documentation & OpenAPI

**Duration**: Day 12  
**Goal**: Generate OpenAPI documentation from Zod schemas

### 7.1 Install OpenAPI Dependencies

```bash
npm install @asteasolutions/zod-to-openapi swagger-ui-express
npm install --save-dev @types/swagger-ui-express
```

### 7.2 OpenAPI Configuration

Create `src/api/docs/openapi.ts`:
```typescript
import { OpenAPIRegistry, OpenApiGeneratorV3 } from '@asteasolutions/zod-to-openapi';
import { z } from 'zod';

export const registry = new OpenAPIRegistry();

// Register security scheme
registry.registerComponent('securitySchemes', 'bearerAuth', {
  type: 'http',
  scheme: 'bearer',
  bearerFormat: 'JWT',
});

// Common response schemas
const apiResponseSchema = z.object({
  status: z.enum(['success', 'error']),
  message: z.string(),
  data: z.unknown().nullable(),
});

registry.register('ApiResponse', apiResponseSchema);

// Generate OpenAPI document
export function generateOpenApiDocument() {
  const generator = new OpenApiGeneratorV3(registry.definitions);

  return generator.generateDocument({
    openapi: '3.0.3',
    info: {
      title: 'Auth Service API',
      version: '2.0.0',
      description: 'Modern authentication microservice with JWT-based authentication',
      contact: {
        name: 'Fernando Barroso',
        email: 'your-email@example.com',
      },
      license: {
        name: 'MIT',
        url: 'https://opensource.org/licenses/MIT',
      },
    },
    servers: [
      {
        url: 'http://localhost:8080',
        description: 'Development server',
      },
    ],
    tags: [
      { name: 'Authentication', description: 'Authentication endpoints' },
      { name: 'Users', description: 'User management endpoints' },
      { name: 'Health', description: 'Health check endpoints' },
    ],
  });
}
```

### 7.3 Register Routes in OpenAPI

Create `src/api/docs/auth.docs.ts`:
```typescript
import { z } from 'zod';
import { registry } from './openapi.js';
import { loginSchema, refreshTokenSchema } from '../../schemas/auth.schema.js';

// Login endpoint
registry.registerPath({
  method: 'post',
  path: '/v1/auth/login',
  tags: ['Authentication'],
  summary: 'User login',
  description: 'Authenticate user with email and password',
  request: {
    body: {
      content: {
        'application/json': {
          schema: loginSchema.shape.body,
        },
      },
    },
  },
  responses: {
    200: {
      description: 'Login successful',
      content: {
        'application/json': {
          schema: z.object({
            status: z.literal('success'),
            message: z.string(),
            data: z.object({
              access_token: z.string(),
              refresh_token: z.string(),
              token_type: z.literal('bearer'),
              expires_in: z.number(),
              user: z.object({
                id: z.string().uuid(),
                email: z.string().email(),
                role: z.enum(['user', 'admin']),
                isActive: z.boolean(),
                createdAt: z.string().datetime(),
                updatedAt: z.string().datetime(),
              }),
            }),
          }),
        },
      },
    },
    401: {
      description: 'Invalid credentials',
    },
    423: {
      description: 'Account locked',
    },
  },
});

// Token refresh endpoint
registry.registerPath({
  method: 'post',
  path: '/v1/auth/token/refresh',
  tags: ['Authentication'],
  summary: 'Refresh access token',
  request: {
    body: {
      content: {
        'application/json': {
          schema: refreshTokenSchema.shape.body,
        },
      },
    },
  },
  responses: {
    200: {
      description: 'Token refreshed successfully',
    },
    401: {
      description: 'Invalid refresh token',
    },
  },
});

// Token validation endpoint
registry.registerPath({
  method: 'post',
  path: '/v1/auth/token/validate',
  tags: ['Authentication'],
  summary: 'Validate access token',
  security: [{ bearerAuth: [] }],
  responses: {
    200: {
      description: 'Token is valid',
    },
    401: {
      description: 'Token is invalid or expired',
    },
  },
});
```

### 7.4 Swagger UI Setup

Update `src/app.ts` to include Swagger UI:
```typescript
import swaggerUi from 'swagger-ui-express';
import { generateOpenApiDocument } from './api/docs/openapi.js';

// After other middleware setup
if (config.server.isDevelopment) {
  const openApiDoc = generateOpenApiDocument();
  app.use('/api-docs', swaggerUi.serve, swaggerUi.setup(openApiDoc));
  app.get('/api-docs.json', (req, res) => res.json(openApiDoc));
}
```

---

## Phase 8: DevOps & Polish

**Duration**: Days 13-14  
**Goal**: Docker, CI/CD, and final polish

### 8.1 Multi-Stage Dockerfile

Create new `Dockerfile`:
```dockerfile
# Stage 1: Builder
FROM node:22-alpine AS builder

WORKDIR /app

# Install dependencies
COPY package*.json ./
RUN npm ci

# Copy source and build
COPY tsconfig*.json ./
COPY src ./src
RUN npm run build

# Prune dev dependencies
RUN npm prune --production

# Stage 2: Runner
FROM node:22-alpine AS runner

WORKDIR /app

# Create non-root user
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nodejs

# Copy built application
COPY --from=builder --chown=nodejs:nodejs /app/dist ./dist
COPY --from=builder --chown=nodejs:nodejs /app/node_modules ./node_modules
COPY --from=builder --chown=nodejs:nodejs /app/package.json ./

# Copy migrations
COPY --chown=nodejs:nodejs migrations ./migrations

USER nodejs

EXPOSE 8080

ENV NODE_ENV=production

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/live || exit 1

CMD ["node", "dist/server.js"]
```

### 8.2 Docker Compose for Development

Create `docker-compose.yml`:
```yaml
version: '3.8'

services:
  auth-service:
    build:
      context: .
      target: builder
    ports:
      - "8080:8080"
    environment:
      NODE_ENV: development
      PORT: 8080
      DATABASE_HOST: postgres
      DATABASE_PORT: 5432
      DATABASE_NAME: auth_db
      DATABASE_USER: auth_user
      DATABASE_PASSWORD: development_password
      JWT_SECRET: development-jwt-secret-key-at-least-32-characters
      LOG_LEVEL: debug
      LOG_PRETTY: "true"
    volumes:
      - ./src:/app/src:ro
    depends_on:
      postgres:
        condition: service_healthy
    command: npm run dev

  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: auth_user
      POSTGRES_PASSWORD: development_password
      POSTGRES_DB: auth_db
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U auth_user -d auth_db"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

### 8.3 ESLint Configuration

Create `eslint.config.js`:
```javascript
import eslint from '@eslint/js';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  eslint.configs.recommended,
  ...tseslint.configs.strictTypeChecked,
  ...tseslint.configs.stylisticTypeChecked,
  {
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/explicit-function-return-type': 'off',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-floating-promises': 'error',
      '@typescript-eslint/no-misused-promises': 'error',
    },
  },
  {
    ignores: ['dist/**', 'node_modules/**', '*.config.js'],
  }
);
```

### 8.4 GitHub Actions CI/CD

Create `.github/workflows/ci.yml`:
```yaml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  build-and-test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: auth_user
          POSTGRES_PASSWORD: test_password
          POSTGRES_DB: auth_db
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Type check
        run: npm run typecheck

      - name: Lint
        run: npm run lint

      - name: Run tests
        run: npm run test:coverage
        env:
          NODE_ENV: test
          DATABASE_HOST: localhost
          DATABASE_PORT: 5432
          DATABASE_NAME: auth_db
          DATABASE_USER: auth_user
          DATABASE_PASSWORD: test_password
          JWT_SECRET: test-secret-key-at-least-32-characters-long

      - name: Build
        run: npm run build

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: ./coverage/lcov.info
          fail_ci_if_error: false

  docker-build:
    runs-on: ubuntu-latest
    needs: build-and-test

    steps:
      - uses: actions/checkout@v4

      - name: Build Docker image
        run: docker build -t auth-service:${{ github.sha }} .

      - name: Test Docker image
        run: |
          docker run --rm -d --name auth-test \
            -e JWT_SECRET=test-secret-key-at-least-32-characters-long \
            -e DATABASE_PASSWORD=test \
            -e NODE_ENV=test \
            auth-service:${{ github.sha }}
          sleep 5
          docker logs auth-test
          docker stop auth-test
```

### 8.5 Update .gitignore

```gitignore
# Dependencies
node_modules/

# Build output
dist/

# Environment files
.env
.env.local
.env.*.local

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Logs
*.log
npm-debug.log*

# Test coverage
coverage/

# TypeScript cache
*.tsbuildinfo
```

---

## File Migration Checklist

Use this checklist to track migration progress:

### Config & Types
- [ ] `src/config/env.ts` - Environment validation
- [ ] `src/config/index.ts` - Configuration export
- [ ] `src/types/user.types.ts` - User types
- [ ] `src/types/auth.types.ts` - Auth types
- [ ] `src/types/api.types.ts` - API types
- [ ] `src/types/express.d.ts` - Express extensions
- [ ] `src/types/index.ts` - Barrel export

### Infrastructure
- [ ] `src/infrastructure/logging/logger.ts` - Pino setup
- [ ] `src/infrastructure/database/connection.ts` - Database pool
- [ ] `src/infrastructure/database/migrations.ts` - Migration runner
- [ ] `src/infrastructure/metrics/prometheus.ts` - Metrics

### Domain
- [ ] `src/domain/entities/User.ts` - User entity
- [ ] `src/domain/repositories/IUserRepository.ts` - Repository interface
- [ ] `src/domain/repositories/UserRepository.ts` - PostgreSQL implementation
- [ ] `src/domain/services/TokenService.ts` - JWT handling
- [ ] `src/domain/services/AuthService.ts` - Authentication logic
- [ ] `src/domain/services/UserService.ts` - User management

### Schemas
- [ ] `src/schemas/auth.schema.ts` - Auth validation
- [ ] `src/schemas/user.schema.ts` - User validation
- [ ] `src/schemas/common.schema.ts` - Shared schemas
- [ ] `src/schemas/index.ts` - Barrel export

### API Layer
- [ ] `src/api/middleware/auth.middleware.ts` - Authentication
- [ ] `src/api/middleware/error.middleware.ts` - Error handling
- [ ] `src/api/middleware/validation.middleware.ts` - Request validation
- [ ] `src/api/middleware/rateLimit.middleware.ts` - Rate limiting
- [ ] `src/api/middleware/requestId.middleware.ts` - Request ID
- [ ] `src/api/controllers/AuthController.ts` - Auth endpoints
- [ ] `src/api/controllers/UserController.ts` - User endpoints
- [ ] `src/api/controllers/HealthController.ts` - Health endpoints
- [ ] `src/api/routes/auth.routes.ts` - Auth routes
- [ ] `src/api/routes/user.routes.ts` - User routes
- [ ] `src/api/routes/health.routes.ts` - Health routes
- [ ] `src/api/routes/index.ts` - Route aggregator

### Application
- [ ] `src/utils/errors.ts` - Custom errors
- [ ] `src/utils/response.ts` - Response helpers
- [ ] `src/app.ts` - Express setup
- [ ] `src/server.ts` - Entry point

### Tests
- [ ] `src/__tests__/setup.ts` - Test setup
- [ ] `src/__tests__/factories/user.factory.ts` - Test factories
- [ ] `src/__tests__/mocks/userRepository.mock.ts` - Mock repository
- [ ] `src/domain/services/__tests__/AuthService.test.ts`
- [ ] `src/domain/services/__tests__/UserService.test.ts`
- [ ] `src/domain/services/__tests__/TokenService.test.ts`
- [ ] `src/api/routes/__tests__/auth.routes.test.ts`
- [ ] `src/api/routes/__tests__/user.routes.test.ts`

### Documentation
- [ ] `src/api/docs/openapi.ts` - OpenAPI config
- [ ] `src/api/docs/auth.docs.ts` - Auth docs
- [ ] `src/api/docs/user.docs.ts` - User docs

### DevOps
- [ ] `Dockerfile` - Multi-stage build
- [ ] `docker-compose.yml` - Development setup
- [ ] `.github/workflows/ci.yml` - CI pipeline
- [ ] `eslint.config.js` - Linting
- [ ] `vitest.config.ts` - Test config
- [ ] `.env.example` - Environment template
- [ ] `.nvmrc` - Node version
- [ ] `.gitignore` - Updated ignores

---

## Dependencies Reference

### Production Dependencies
```json
{
  "dependencies": {
    "@asteasolutions/zod-to-openapi": "^7.0.0",
    "bcrypt": "^5.1.1",
    "express": "^4.21.0",
    "express-rate-limit": "^7.4.0",
    "helmet": "^7.1.0",
    "jsonwebtoken": "^9.0.2",
    "pg": "^8.12.0",
    "pino": "^9.0.0",
    "pino-pretty": "^11.0.0",
    "prom-client": "^15.1.0",
    "swagger-ui-express": "^5.0.0",
    "uuid": "^10.0.0",
    "zod": "^3.23.0"
  }
}
```

### Development Dependencies
```json
{
  "devDependencies": {
    "@eslint/js": "^9.0.0",
    "@types/bcrypt": "^5.0.2",
    "@types/express": "^4.17.21",
    "@types/jsonwebtoken": "^9.0.6",
    "@types/node": "^22.0.0",
    "@types/pg": "^8.11.0",
    "@types/supertest": "^6.0.0",
    "@types/swagger-ui-express": "^4.1.0",
    "@types/uuid": "^10.0.0",
    "@vitest/coverage-v8": "^2.0.0",
    "rimraf": "^5.0.0",
    "supertest": "^7.0.0",
    "tsx": "^4.0.0",
    "typescript": "^5.5.0",
    "typescript-eslint": "^8.0.0",
    "vitest": "^2.0.0"
  }
}
```

---

## Success Criteria

The migration is complete when:

1. ✅ All TypeScript files compile without errors (`npm run typecheck`)
2. ✅ All linting rules pass (`npm run lint`)
3. ✅ All tests pass with >70% coverage (`npm run test:coverage`)
4. ✅ Docker image builds successfully
5. ✅ API documentation is accessible at `/api-docs`
6. ✅ All existing functionality works as before
7. ✅ CI pipeline passes on all checks

---

## Next Steps After Migration

1. **Add more features**: Password reset, email verification, 2FA
2. **Add Redis**: Session management, token blacklisting
3. **Add OpenTelemetry**: Distributed tracing
4. **Add Sentry**: Error monitoring
5. **Add API rate limiting per user**: Beyond IP-based limits
6. **Add refresh token rotation**: Security enhancement
7. **Add audit logging**: Track all auth events

---

*Document Version: 1.0*  
*Created: January 2026*  
*Author: AI-Assisted Migration Plan*

