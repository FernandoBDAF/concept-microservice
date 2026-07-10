import bcrypt from "bcrypt";
import { v4 as uuidv4 } from "uuid";
import crypto from "crypto";
import User from "../models/User.js";
import db from "../service/databaseService.js";

class UserRepository {
  async createUser(userData) {
    const id = uuidv4();
    const salt = crypto.randomBytes(32).toString("hex");
    const hashedPassword = await bcrypt.hash(userData.password + salt, 12);

    const query = `
      INSERT INTO users (id, email, hashed_password, salt, role, is_active, created_at, updated_at)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
      RETURNING *
    `;

    const now = new Date();
    const result = await db.query(query, [
      id,
      userData.email.toLowerCase(),
      hashedPassword,
      salt,
      userData.role || "user",
      true,
      now,
      now,
    ]);

    return this._mapToUser(result.rows[0]);
  }

  async getUserByEmail(email) {
    const query = "SELECT * FROM users WHERE email = $1";
    const result = await db.query(query, [email.toLowerCase()]);

    if (result.rows.length === 0) {
      return null;
    }

    return this._mapToUser(result.rows[0]);
  }

  async getUserById(id) {
    const query = "SELECT * FROM users WHERE id = $1";
    const result = await db.query(query, [id]);

    if (result.rows.length === 0) {
      return null;
    }

    return this._mapToUser(result.rows[0]);
  }

  async updateUser(id, userData) {
    const updates = [];
    const values = [];
    let paramCount = 1;

    if (userData.email) {
      updates.push(`email = $${paramCount++}`);
      values.push(userData.email.toLowerCase());
    }

    if (userData.role) {
      updates.push(`role = $${paramCount++}`);
      values.push(userData.role);
    }

    if (userData.isActive !== undefined) {
      updates.push(`is_active = $${paramCount++}`);
      values.push(userData.isActive);
    }

    if (userData.password) {
      const salt = crypto.randomBytes(32).toString("hex");
      const hashedPassword = await bcrypt.hash(userData.password + salt, 12);
      updates.push(`hashed_password = $${paramCount++}`);
      values.push(hashedPassword);
      updates.push(`salt = $${paramCount++}`);
      values.push(salt);
    }

    updates.push(`updated_at = $${paramCount++}`);
    values.push(new Date());

    values.push(id);

    const query = `
      UPDATE users 
      SET ${updates.join(", ")}
      WHERE id = $${paramCount}
      RETURNING *
    `;

    const result = await db.query(query, values);

    if (result.rows.length === 0) {
      return null;
    }

    return this._mapToUser(result.rows[0]);
  }

  async deleteUser(id) {
    const query = "DELETE FROM users WHERE id = $1 RETURNING *";
    const result = await db.query(query, [id]);
    return result.rows.length > 0;
  }

  async listUsers(page = 1, pageSize = 10) {
    const offset = (page - 1) * pageSize;
    const query = `
      SELECT * FROM users 
      ORDER BY created_at DESC 
      LIMIT $1 OFFSET $2
    `;

    const result = await db.query(query, [pageSize, offset]);
    return result.rows.map((row) => this._mapToUser(row));
  }

  async validatePassword(user, password) {
    return bcrypt.compare(password + user.salt, user.hashedPassword);
  }

  async recordLoginAttempt(userId, success) {
    const query = `
      UPDATE users 
      SET 
        failed_attempts = CASE 
          WHEN $2 = true THEN 0 
          ELSE failed_attempts + 1 
        END,
        locked_until = CASE 
          WHEN $2 = false AND failed_attempts >= 4 THEN NOW() + INTERVAL '30 minutes'
          WHEN $2 = true THEN NULL
          ELSE locked_until 
        END,
        updated_at = NOW()
      WHERE id = $1
      RETURNING *
    `;

    const result = await db.query(query, [userId, success]);
    return result.rows.length > 0 ? this._mapToUser(result.rows[0]) : null;
  }

  _mapToUser(row) {
    return new User(
      row.id,
      row.email,
      row.hashed_password,
      row.salt,
      row.role,
      row.is_active,
      row.created_at,
      row.updated_at
    );
  }
}

export default new UserRepository();
