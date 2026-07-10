import bcrypt from "bcrypt";
import crypto from "node:crypto";
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

export class UserRepository implements IUserRepository {
  private readonly SALT_ROUNDS = 12;
  private readonly log = logger.child({ repository: "UserRepository" });

  async create(data: CreateUserDTO): Promise<UserEntity> {
    const id = uuidv4();
    const salt = crypto.randomBytes(32).toString("hex");
    const hashedPassword = await bcrypt.hash(
      data.password + salt,
      this.SALT_ROUNDS
    );
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
      data.role ?? "user",
      true,
      now,
      now,
    ]);

    this.log.info({ userId: id, email: data.email }, "User created");
    return UserEntity.fromRow(result.rows[0]!);
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
      const salt = crypto.randomBytes(32).toString("hex");
      const hashedPassword = await bcrypt.hash(
        data.password + salt,
        this.SALT_ROUNDS
      );
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
      UPDATE users SET ${updates.join(", ")}
      WHERE id = $${paramCount}
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
    return bcrypt.compare(password + user.salt, user.hashedPassword);
  }
}

export const userRepository = new UserRepository();

