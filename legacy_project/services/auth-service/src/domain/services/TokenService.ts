import jwt from "jsonwebtoken";
import { v4 as uuidv4 } from "uuid";
import type { TokenPair, TokenPayload } from "../../types/index.js";
import type { UserEntity } from "../entities/User.js";
import { config } from "../../config/index.js";
import { UnauthorizedError } from "../../utils/errors.js";
import { logger } from "../../infrastructure/logging/logger.js";

export class TokenService {
  private readonly log = logger.child({ service: "TokenService" });

  generateTokens(user: UserEntity): TokenPair {
    const jti = uuidv4();

    const basePayload = {
      userId: user.id,
      email: user.email,
      role: user.role,
      jti,
    };

    const accessToken = jwt.sign(
      { ...basePayload, tokenType: "ACCESS_TOKEN" as const },
      config.jwt.secret,
      { expiresIn: config.jwt.accessTokenExpiry, algorithm: "HS256" }
    );

    const refreshToken = jwt.sign(
      { ...basePayload, tokenType: "REFRESH_TOKEN" as const },
      config.jwt.secret,
      { expiresIn: config.jwt.refreshTokenExpiry, algorithm: "HS256" }
    );

    this.log.debug({ userId: user.id, jti }, "Tokens generated");

    return {
      accessToken,
      refreshToken,
      tokenType: "bearer",
      expiresIn: this.parseExpiryToSeconds(config.jwt.accessTokenExpiry),
    };
  }

  verifyAccessToken(token: string): TokenPayload {
    try {
      const decoded = jwt.verify(token, config.jwt.secret, {
        algorithms: ["HS256"],
      }) as TokenPayload;

      if (decoded.tokenType !== "ACCESS_TOKEN") {
        throw new UnauthorizedError("Invalid token type");
      }

      return decoded;
    } catch (error) {
      if (error instanceof jwt.TokenExpiredError) {
        throw new UnauthorizedError("Token expired");
      }
      if (error instanceof jwt.JsonWebTokenError) {
        throw new UnauthorizedError("Invalid token");
      }
      throw error;
    }
  }

  verifyRefreshToken(token: string): TokenPayload {
    try {
      const decoded = jwt.verify(token, config.jwt.secret, {
        algorithms: ["HS256"],
      }) as TokenPayload;

      if (decoded.tokenType !== "REFRESH_TOKEN") {
        throw new UnauthorizedError("Invalid refresh token");
      }

      return decoded;
    } catch (error) {
      if (error instanceof jwt.TokenExpiredError) {
        throw new UnauthorizedError("Refresh token expired");
      }
      if (error instanceof jwt.JsonWebTokenError) {
        throw new UnauthorizedError("Invalid refresh token");
      }
      throw error;
    }
  }

  decodeToken(token: string): TokenPayload | null {
    return jwt.decode(token) as TokenPayload | null;
  }

  private parseExpiryToSeconds(expiry: string): number {
    const match = expiry.match(/^(\d+)([smhd])$/);
    if (!match) return 3600;

    const [, value, unit] = match;
    const num = parseInt(value ?? "0", 10);

    switch (unit) {
      case "s":
        return num;
      case "m":
        return num * 60;
      case "h":
        return num * 3600;
      case "d":
        return num * 86400;
      default:
        return 3600;
    }
  }
}

export const tokenService = new TokenService();

