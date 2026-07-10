import type {
  LoginRequest,
  LoginResponse,
  TokenPair,
  TokenValidationResult,
} from "../../types/index.js";
import type { IUserRepository } from "../repositories/IUserRepository.js";
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
    private readonly tokenService: TokenService
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

      if (!user || !user.isActive) {
        return { valid: false, error: "User not found or inactive" };
      }

      return { valid: true, user: user.toSafeUser() };
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Token validation failed";
      return { valid: false, error: message };
    }
  }

  async refreshTokens(refreshToken: string): Promise<TokenPair> {
    const decoded = this.tokenService.verifyRefreshToken(refreshToken);
    const user = await this.userRepository.findById(decoded.userId);

    if (!user || !user.isActive) {
      throw new UnauthorizedError("User not found or inactive");
    }

    this.log.info({ userId: user.id }, "Token refreshed");
    return this.tokenService.generateTokens(user);
  }

  async logout(token: string): Promise<void> {
    const decoded = this.tokenService.decodeToken(token);
    if (decoded) {
      this.log.info(
        { userId: decoded.userId, jti: decoded.jti },
        "User logged out"
      );
    }
  }
}

