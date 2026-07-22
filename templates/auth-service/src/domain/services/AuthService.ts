import type {
  LoginRequest,
  LoginResponse,
  TokenPair,
  TokenValidationResult,
} from "../../types/index.js";
import type { IUserRepository } from "../repositories/IUserRepository.js";
import type { ISessionRepository } from "../repositories/ISessionRepository.js";
import type { IAuditLogRepository } from "../repositories/IAuditLogRepository.js";
import { TokenService } from "./TokenService.js";
import {
  AccountLockedError,
  UnauthorizedError,
} from "../../utils/errors.js";
import { logger } from "../../infrastructure/logging/logger.js";

export class AuthService {
  private readonly log = logger.child({ service: "AuthService" });

  constructor(
    private readonly userRepository: IUserRepository,
    private readonly tokenService: TokenService,
    private readonly sessionRepository: ISessionRepository,
    private readonly auditLogRepository: IAuditLogRepository
  ) {}

  async login(
    request: LoginRequest,
    clientInfo?: { ip?: string; userAgent?: string }
  ): Promise<LoginResponse> {
    const { email, password } = request;
    this.log.info({ email, clientInfo }, "Login attempt");

    const user = await this.userRepository.findByEmail(email);
    if (!user) {
      this.log.warn({ email, reason: "USER_NOT_FOUND" }, "Login failed");
      throw new UnauthorizedError("Invalid credentials");
    }

    if (user.isLocked()) {
      this.log.warn(
        { userId: user.id, lockedUntil: user.lockedUntil },
        "Login attempt on locked account"
      );
      throw new AccountLockedError(user.lockedUntil ?? undefined);
    }

    if (!user.isActive) {
      this.log.warn({ userId: user.id }, "Login attempt on inactive account");
      throw new UnauthorizedError("Account is inactive");
    }

    const isValid = await this.userRepository.validatePassword(user, password);
    if (!isValid) {
      await this.userRepository.recordLoginAttempt(user.id, false);
      this.log.warn(
        { userId: user.id, reason: "INVALID_PASSWORD" },
        "Login failed"
      );
      throw new UnauthorizedError("Invalid credentials");
    }

    await this.userRepository.recordLoginAttempt(user.id, true);
    const tokens = this.tokenService.generateTokens(user);

    // Open a rotating refresh session (ADR-009.2): the refresh jti is the
    // current token id; rotation/reuse is tracked against it.
    const { jti, expiresAt } = this.sessionFieldsFrom(tokens);
    await this.sessionRepository.create(user.id, jti, expiresAt);

    this.log.info({ userId: user.id }, "Login successful");

    return {
      user: user.toSafeUser(),
      tokens,
    };
  }

  async validateToken(token: string): Promise<TokenValidationResult> {
    try {
      const decoded = this.tokenService.verifyAccessToken(token);
      const user = await this.userRepository.findById(decoded.userId);

      if (!user?.isActive) {
        return { valid: false, error: "User not found or inactive" };
      }

      return { valid: true, user: user.toSafeUser() };
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Token validation failed";
      return { valid: false, error: message };
    }
  }

  /**
   * One-time-use refresh with rotation + reuse detection (ADR-009.2).
   * - presented jti == session.refresh_token_id  -> rotate (new jti)
   * - presented jti == some session.previous_token_id (already rotated out) or
   *   the current token of an already-revoked session -> reuse (theft signal):
   *   revoke the session + write an audit entry, reject.
   */
  async refreshTokens(refreshToken: string): Promise<TokenPair> {
    const decoded = this.tokenService.verifyRefreshToken(refreshToken);
    const presentedJti = decoded.jti;

    const session =
      await this.sessionRepository.findByRefreshTokenId(presentedJti);

    // The presented jti is not the current token of any live session: either
    // it was already rotated out (reuse) or belongs to a revoked session.
    if (!session || session.revokedAt !== null) {
      await this.handlePossibleReuse(presentedJti, session?.id ?? null);
      throw new UnauthorizedError("Invalid refresh token");
    }

    if (session.expiresAt.getTime() <= Date.now()) {
      throw new UnauthorizedError("Refresh token expired");
    }

    const user = await this.userRepository.findById(decoded.userId);
    if (!user?.isActive) {
      throw new UnauthorizedError("User not found or inactive");
    }

    const tokens = this.tokenService.generateTokens(user);
    const { jti: newJti, expiresAt } = this.sessionFieldsFrom(tokens);
    await this.sessionRepository.rotate(
      session.id,
      newJti,
      presentedJti,
      expiresAt
    );

    this.log.info({ userId: user.id, sessionId: session.id }, "Token refreshed");
    return tokens;
  }

  async logout(token: string): Promise<void> {
    const decoded = this.tokenService.decodeToken(token);
    if (!decoded?.jti) return;

    const session = await this.sessionRepository.findByRefreshTokenId(
      decoded.jti
    );
    if (session && session.revokedAt === null) {
      await this.sessionRepository.revoke(session.id);
    }

    this.log.info(
      { userId: decoded.userId, jti: decoded.jti },
      "User logged out"
    );
  }

  /**
   * A jti that is not a live current token may be a rotated-out ("previous")
   * token being replayed — a theft signal. Revoke the whole chain and audit.
   */
  private async handlePossibleReuse(
    presentedJti: string,
    knownSessionId: string | null
  ): Promise<void> {
    const reused =
      await this.sessionRepository.findByPreviousTokenId(presentedJti);
    const sessionId = reused?.id ?? knownSessionId;
    if (!sessionId) return;

    if (reused && reused.revokedAt === null) {
      await this.sessionRepository.revoke(sessionId);
    }

    this.log.warn(
      { sessionId, userId: reused?.userId, jti: presentedJti },
      "Refresh token reuse detected"
    );
    await this.auditLogRepository.record({
      userId: reused?.userId ?? null,
      action: "REFRESH_TOKEN_REUSE",
      success: false,
      details: { sessionId, presentedJti },
    });
  }

  /** jti + absolute expiry of a freshly minted refresh token. */
  private sessionFieldsFrom(tokens: TokenPair): {
    jti: string;
    expiresAt: Date;
  } {
    const decoded = this.tokenService.decodeToken(tokens.refreshToken);
    if (!decoded?.jti || !decoded.exp) {
      throw new UnauthorizedError("Failed to issue refresh token");
    }
    return { jti: decoded.jti, expiresAt: new Date(decoded.exp * 1000) };
  }
}
