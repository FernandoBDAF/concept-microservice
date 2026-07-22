import bcrypt from "bcrypt";
import { v4 as uuidv4 } from "uuid";
import type { IUserRepository } from "./IUserRepository.js";
import type {
  CreateUserDTO,
  PaginatedResult,
  PaginationParams,
  UpdateUserDTO,
} from "../../types/index.js";
import { UserEntity, type UserRow } from "../entities/User.js";
import { db } from "../../infrastructure/database/connection.js";
import { config } from "../../config/index.js";
import { logger } from "../../infrastructure/logging/logger.js";
import { DatabaseError } from "../../utils/errors.js";

export class UserRepository implements IUserRepository {
  private readonly SALT_ROUNDS = 12;
  private readonly log = logger.child({ repository: "UserRepository" });

  async create(data: CreateUserDTO): Promise<UserEntity> {
    const id = uuidv4();
    // bcrypt salts internally (ADR-009.6: the separate salt column was dropped).
    const hashedPassword = await bcrypt.hash(data.password, this.SALT_ROUNDS);
    const now = new Date();

    const query = `
      INSERT INTO users (id, email, hashed_password, role, is_active, created_at, updated_at)
      VALUES ($1, $2, $3, $4, $5, $6, $7)
      RETURNING *
    `;

    const result = await db.query<UserRow>(query, [
      id,
      data.email.toLowerCase().trim(),
      hashedPassword,
      data.role ?? "user",
      true,
      now,
      now,
    ]);

    const row = result.rows[0];
    if (!row) {
      throw new DatabaseError("Failed to create user");
    }

    this.log.info({ userId: id, email: data.email }, "User created");
    return UserEntity.fromRow(row);
  }

  async findById(id: string): Promise<UserEntity | null> {
    const result = await db.query<UserRow>(
      "SELECT * FROM users WHERE id = $1",
      [id]
    );
    return result.rows[0] ? UserEntity.fromRow(result.rows[0]) : null;
  }

  async findByEmail(email: string): Promise<UserEntity | null> {
    const result = await db.query<UserRow>(
      "SELECT * FROM users WHERE email = $1",
      [email.toLowerCase().trim()]
    );
    return result.rows[0] ? UserEntity.fromRow(result.rows[0]) : null;
  }

  async update(id: string, data: UpdateUserDTO): Promise<UserEntity | null> {
    const updates: string[] = [];
    const values: unknown[] = [];
    let paramCount = 1;
    const nextParam = () => String(paramCount++);

    if (data.email !== undefined) {
      updates.push(`email = $${nextParam()}`);
      values.push(data.email.toLowerCase().trim());
    }

    if (data.role !== undefined) {
      updates.push(`role = $${nextParam()}`);
      values.push(data.role);
    }

    if (data.isActive !== undefined) {
      updates.push(`is_active = $${nextParam()}`);
      values.push(data.isActive);
    }

    if (data.password !== undefined) {
      const hashedPassword = await bcrypt.hash(data.password, this.SALT_ROUNDS);
      updates.push(`hashed_password = $${nextParam()}`);
      values.push(hashedPassword);
    }

    if (updates.length === 0) {
      return this.findById(id);
    }

    updates.push(`updated_at = $${nextParam()}`);
    values.push(new Date());
    values.push(id);
    const idParam = String(paramCount);

    const query = `
      UPDATE users SET ${updates.join(", ")}
      WHERE id = $${idParam}
      RETURNING *
    `;

    const result = await db.query<UserRow>(query, values);
    if (!result.rows[0]) return null;

    this.log.info({ userId: id }, "User updated");
    return UserEntity.fromRow(result.rows[0]);
  }

  async delete(id: string): Promise<boolean> {
    const result = await db.query("DELETE FROM users WHERE id = $1 RETURNING id", [
      id,
    ]);
    const deleted = result.rowCount !== null && result.rowCount > 0;
    if (deleted) {
      this.log.info({ userId: id }, "User deleted");
    }
    return deleted;
  }

  async list(params: PaginationParams): Promise<PaginatedResult<UserEntity>> {
    const { page, pageSize } = params;
    const offset = (page - 1) * pageSize;

    const [dataResult, countResult] = await Promise.all([
      db.query<UserRow>(
        "SELECT * FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2",
        [pageSize, offset]
      ),
      db.query<{ count: string }>("SELECT COUNT(*) as count FROM users"),
    ]);

    const total = parseInt(countResult.rows[0]?.count ?? "0", 10);

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

  async recordLoginAttempt(
    id: string,
    success: boolean
  ): Promise<UserEntity | null> {
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

    const result = await db.query<UserRow>(query, [
      id,
      success,
      maxAttempts - 1,
      lockoutDuration,
    ]);
    return result.rows[0] ? UserEntity.fromRow(result.rows[0]) : null;
  }

  async validatePassword(user: UserEntity, password: string): Promise<boolean> {
    return bcrypt.compare(password, user.hashedPassword);
  }
}

export const userRepository = new UserRepository();

