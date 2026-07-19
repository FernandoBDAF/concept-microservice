import type {
  ISessionRepository,
  SessionRecord,
} from "./ISessionRepository.js";
import { db } from "../../infrastructure/database/connection.js";
import { logger } from "../../infrastructure/logging/logger.js";
import { DatabaseError } from "../../utils/errors.js";

interface SessionRow {
  id: string;
  user_id: string;
  refresh_token_id: string;
  previous_token_id: string | null;
  expires_at: Date;
  created_at: Date;
  revoked_at: Date | null;
}

function toRecord(row: SessionRow): SessionRecord {
  return {
    id: row.id,
    userId: row.user_id,
    refreshTokenId: row.refresh_token_id,
    previousTokenId: row.previous_token_id,
    expiresAt: row.expires_at,
    createdAt: row.created_at,
    revokedAt: row.revoked_at,
  };
}

export class SessionRepository implements ISessionRepository {
  private readonly log = logger.child({ repository: "SessionRepository" });

  async create(
    userId: string,
    refreshTokenId: string,
    expiresAt: Date
  ): Promise<SessionRecord> {
    const result = await db.query<SessionRow>(
      `INSERT INTO sessions (user_id, refresh_token_id, expires_at)
       VALUES ($1, $2, $3)
       RETURNING *`,
      [userId, refreshTokenId, expiresAt]
    );
    const row = result.rows[0];
    if (!row) {
      throw new DatabaseError("Failed to create session");
    }
    this.log.info({ userId, sessionId: row.id }, "Session created");
    return toRecord(row);
  }

  async findByRefreshTokenId(
    refreshTokenId: string
  ): Promise<SessionRecord | null> {
    const result = await db.query<SessionRow>(
      "SELECT * FROM sessions WHERE refresh_token_id = $1",
      [refreshTokenId]
    );
    return result.rows[0] ? toRecord(result.rows[0]) : null;
  }

  async findByPreviousTokenId(
    previousTokenId: string
  ): Promise<SessionRecord | null> {
    const result = await db.query<SessionRow>(
      "SELECT * FROM sessions WHERE previous_token_id = $1",
      [previousTokenId]
    );
    return result.rows[0] ? toRecord(result.rows[0]) : null;
  }

  async rotate(
    id: string,
    newRefreshTokenId: string,
    previousTokenId: string,
    expiresAt: Date
  ): Promise<SessionRecord | null> {
    const result = await db.query<SessionRow>(
      `UPDATE sessions
         SET refresh_token_id = $2,
             previous_token_id = $3,
             expires_at = $4
       WHERE id = $1
       RETURNING *`,
      [id, newRefreshTokenId, previousTokenId, expiresAt]
    );
    if (!result.rows[0]) return null;
    this.log.info({ sessionId: id }, "Session rotated");
    return toRecord(result.rows[0]);
  }

  async revoke(id: string): Promise<void> {
    await db.query(
      "UPDATE sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL",
      [id]
    );
    this.log.info({ sessionId: id }, "Session revoked");
  }
}

export const sessionRepository = new SessionRepository();
