import jwt, { type SignOptions } from "jsonwebtoken";
import { v4 as uuidv4 } from "uuid";
import { createPublicKey, type KeyObject } from "node:crypto";
import type { TokenPair, TokenPayload } from "../../types/index.js";
import type { UserEntity } from "../entities/User.js";
import { config } from "../../config/index.js";
import { UnauthorizedError } from "../../utils/errors.js";
import { logger } from "../../infrastructure/logging/logger.js";
import { JwksService } from "./JwksService.js";

export class TokenService {
  private readonly log = logger.child({ service: "TokenService" });

  // Lazily derived RS256 material (only when a keypair is configured). The
  // signing kid is derived from the public half of the private key so it
  // always matches the kid published in the JWKS document.
  private signingKid: string | null = null;
  private verifyKeys: Map<string, KeyObject> | null = null;

  private getSigningKid(): string {
    if (this.signingKid) return this.signingKid;
    const publicKey = createPublicKey(config.jwt.privateKey as string);
    this.signingKid = JwksService.kidFor(publicKey);
    return this.signingKid;
  }

  /** kid -> public key, for RS256 verification (rotation-ready). */
  private getVerifyKeys(): Map<string, KeyObject> {
    if (this.verifyKeys) return this.verifyKeys;
    const keys = new Map<string, KeyObject>();
    for (const pem of config.jwt.publicKeys) {
      const key = createPublicKey(pem);
      keys.set(JwksService.kidFor(key), key);
    }
    this.verifyKeys = keys;
    return keys;
  }

  generateTokens(user: UserEntity): TokenPair {
    const jti = uuidv4();

    const basePayload = {
      userId: user.id,
      email: user.email,
      role: user.role,
      jti,
    };

    const accessToken = this.sign(
      { ...basePayload, tokenType: "ACCESS_TOKEN" as const },
      config.jwt.accessTokenExpiry
    );
    const refreshToken = this.sign(
      { ...basePayload, tokenType: "REFRESH_TOKEN" as const },
      config.jwt.refreshTokenExpiry
    );

    this.log.debug(
      { userId: user.id, jti, algorithm: config.jwt.algorithm },
      "Tokens generated"
    );

    return {
      accessToken,
      refreshToken,
      tokenType: "bearer",
      expiresIn: this.parseExpiryToSeconds(config.jwt.accessTokenExpiry),
    };
  }

  /** Sign with RS256 (kid header) when a keypair is configured, else HS256. */
  private sign(payload: object, expiresIn: string): string {
    const options: SignOptions = {
      expiresIn: expiresIn as Exclude<SignOptions["expiresIn"], undefined>,
    };

    if (config.jwt.algorithm === "RS256") {
      options.algorithm = "RS256";
      options.keyid = this.getSigningKid();
      return jwt.sign(payload, config.jwt.privateKey as string, options);
    }

    options.algorithm = "HS256";
    return jwt.sign(payload, config.jwt.secret, options);
  }

  /**
   * Verify signature + expiry, accepting BOTH algorithms during migration:
   * try RS256 (by any configured public key) first, then fall back to HS256.
   * A genuine expiry surfaces as an expired error rather than a fallthrough.
   */
  private verifyToken(token: string): TokenPayload {
    const keys = this.getVerifyKeys();
    if (keys.size > 0) {
      for (const key of keys.values()) {
        try {
          return jwt.verify(token, key, {
            algorithms: ["RS256"],
          }) as TokenPayload;
        } catch (error) {
          // A matching key that reports expiry means the token is genuinely
          // expired — do not fall back to HS256 (which would misreport it).
          if (error instanceof jwt.TokenExpiredError) {
            throw new UnauthorizedError("Token expired");
          }
          // Wrong key / different alg (e.g. a legacy HS256 token): try next,
          // then the HS256 path below.
        }
      }
    }

    try {
      return jwt.verify(token, config.jwt.secret, {
        algorithms: ["HS256"],
      }) as TokenPayload;
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

  verifyAccessToken(token: string): TokenPayload {
    const decoded = this.verifyToken(token);
    if (decoded.tokenType !== "ACCESS_TOKEN") {
      throw new UnauthorizedError("Invalid token type");
    }
    return decoded;
  }

  verifyRefreshToken(token: string): TokenPayload {
    const decoded = this.verifyToken(token);
    if (decoded.tokenType !== "REFRESH_TOKEN") {
      throw new UnauthorizedError("Invalid refresh token");
    }
    return decoded;
  }

  decodeToken(token: string): TokenPayload | null {
    return jwt.decode(token) as TokenPayload | null;
  }

  private parseExpiryToSeconds(expiry: string): number {
    const match = /^(\d+)([smhd])$/.exec(expiry);
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
