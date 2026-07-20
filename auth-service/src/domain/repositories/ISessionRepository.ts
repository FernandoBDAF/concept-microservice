/**
 * Refresh-token sessions (ADR-009.2). One row per active refresh chain.
 * `refreshTokenId` is the jti of the CURRENT refresh token; `previousTokenId`
 * is the immediately-rotated-out jti, used to detect reuse (theft signal).
 */
export interface SessionRecord {
  id: string;
  userId: string;
  refreshTokenId: string;
  previousTokenId: string | null;
  expiresAt: Date;
  createdAt: Date;
  revokedAt: Date | null;
}

export interface ISessionRepository {
  /** Open a session for a freshly issued refresh token. */
  create(
    userId: string,
    refreshTokenId: string,
    expiresAt: Date
  ): Promise<SessionRecord>;
  /** The session whose CURRENT refresh token is this jti (or null). */
  findByRefreshTokenId(refreshTokenId: string): Promise<SessionRecord | null>;
  /** The session that rotated this jti out (reuse detection) (or null). */
  findByPreviousTokenId(previousTokenId: string): Promise<SessionRecord | null>;
  /** Advance the chain: new current jti, prior jti recorded, expiry extended. */
  rotate(
    id: string,
    newRefreshTokenId: string,
    previousTokenId: string,
    expiresAt: Date
  ): Promise<SessionRecord | null>;
  /** Mark the session revoked (logout, or reuse-triggered). */
  revoke(id: string): Promise<void>;
}
