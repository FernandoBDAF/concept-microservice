import StorageServiceClient from "../clients/storageServiceClient.js";
import CacheServiceClient from "../clients/cacheServiceClient.js";
import passwordService from "./passwordService.js";
import tokenService from "./tokenService.js";
import config from "../config/config.js";

class AuthenticationService {
  constructor() {
    this.storageClient = new StorageServiceClient(config);
    this.cacheClient = new CacheServiceClient(config);
    this.passwordService = passwordService;
    this.tokenService = tokenService;
  }

  async authenticateUser(email, password, req) {
    const startTime = Date.now();

    try {
      console.log(`Authentication attempt for user: ${email}`);

      // 1. Get user data via storage-service
      const user = await this.storageClient.getUserByEmail(email);

      if (!user) {
        await this._recordFailedAttempt(null, email, req, "USER_NOT_FOUND");
        throw new Error("Invalid credentials");
      }

      // 2. Check if account is locked
      if (user.locked_until && new Date(user.locked_until) > new Date()) {
        await this._recordFailedAttempt(user.id, email, req, "ACCOUNT_LOCKED");
        throw new Error("Account is temporarily locked");
      }

      // 3. Check if account is active
      if (!user.is_active) {
        await this._recordFailedAttempt(
          user.id,
          email,
          req,
          "ACCOUNT_INACTIVE"
        );
        throw new Error("Account is inactive");
      }

      // 4. Validate password locally (Argon2)
      const isValid = await this.passwordService.validatePassword(
        password,
        user.hashed_password,
        user.salt
      );

      if (!isValid) {
        await this._recordFailedAttempt(
          user.id,
          email,
          req,
          "INVALID_PASSWORD"
        );
        throw new Error("Invalid credentials");
      }

      // 5. Generate JWT tokens
      const tokens = await this.tokenService.generateTokens(user);

      // 6. Store session in cache (non-blocking)
      const sessionData = {
        userId: user.id,
        email: user.email,
        role: user.role,
        firstName: user.first_name,
        lastName: user.last_name,
        loginTime: new Date().toISOString(),
      };

      this.cacheClient.storeSession(tokens.jti, sessionData, 3600);

      // 7. Record successful login via storage-service (non-blocking)
      this.storageClient.recordLoginAttempt(user.id, req.ip, true);
      this.storageClient.logAuditEvent({
        user_id: user.id,
        action: "LOGIN_SUCCESS",
        ip_address: req.ip,
        user_agent: req.get("User-Agent"),
        success: true,
        details: JSON.stringify({
          loginTime: new Date().toISOString(),
          tokenId: tokens.jti,
        }),
      });

      // Record metrics
      const duration = Date.now() - startTime;
      console.log(`Authentication successful for ${email} in ${duration}ms`);

      return {
        status: "success",
        message: "Authentication successful",
        data: {
          access_token: tokens.accessToken,
          refresh_token: tokens.refreshToken,
          token_type: "bearer",
          expires_in: 3600,
          user: {
            id: user.id,
            email: user.email,
            firstName: user.first_name,
            lastName: user.last_name,
            role: user.role,
          },
        },
      };
    } catch (error) {
      const duration = Date.now() - startTime;
      console.error(
        `Authentication failed for ${email} in ${duration}ms:`,
        error.message
      );
      throw error;
    }
  }

  async validateToken(token) {
    try {
      // 1. Verify JWT token locally
      const decoded = await this.tokenService.verifyToken(token);

      // 2. Check if token is blacklisted (non-blocking)
      const isBlacklisted = await this.cacheClient.isTokenBlacklisted(
        decoded.jti
      );
      if (isBlacklisted) {
        throw new Error("Token has been revoked");
      }

      // 3. Get session data from cache (optional)
      const sessionData = await this.cacheClient.getSession(decoded.jti);

      return {
        valid: true,
        user: {
          id: decoded.userId,
          email: decoded.email,
          role: decoded.role,
          firstName: decoded.firstName,
          lastName: decoded.lastName,
        },
        session: sessionData,
      };
    } catch (error) {
      console.error("Token validation failed:", error.message);
      return {
        valid: false,
        error: error.message,
      };
    }
  }

  async refreshToken(refreshToken) {
    try {
      // 1. Verify refresh token
      const decoded = await this.tokenService.verifyRefreshToken(refreshToken);

      // 2. Get user data via storage-service
      const user = await this.storageClient.getUserById(decoded.userId);

      if (!user || !user.is_active) {
        throw new Error("User not found or inactive");
      }

      // 3. Generate new tokens
      const tokens = await this.tokenService.generateTokens(user);

      // 4. Blacklist old token
      await this.cacheClient.blacklistToken(decoded.jti, 3600);

      // 5. Store new session
      const sessionData = {
        userId: user.id,
        email: user.email,
        role: user.role,
        refreshTime: new Date().toISOString(),
      };

      this.cacheClient.storeSession(tokens.jti, sessionData, 3600);

      return {
        status: "success",
        message: "Token refreshed successfully",
        data: {
          access_token: tokens.accessToken,
          refresh_token: tokens.refreshToken,
          token_type: "bearer",
          expires_in: 3600,
        },
      };
    } catch (error) {
      console.error("Token refresh failed:", error.message);
      throw error;
    }
  }

  async logout(token) {
    try {
      const decoded = await this.tokenService.verifyToken(token);

      // Blacklist token
      await this.cacheClient.blacklistToken(decoded.jti, 3600);

      // Invalidate session
      await this.cacheClient.invalidateSession(decoded.jti);

      // Log audit event
      this.storageClient.logAuditEvent({
        user_id: decoded.userId,
        action: "LOGOUT",
        ip_address: "unknown", // Will need to be passed from request
        success: true,
        details: JSON.stringify({
          logoutTime: new Date().toISOString(),
          tokenId: decoded.jti,
        }),
      });

      return {
        status: "success",
        message: "Logged out successfully",
      };
    } catch (error) {
      console.error("Logout failed:", error.message);
      throw error;
    }
  }

  // Private method for recording failed attempts
  async _recordFailedAttempt(userId, email, req, reason) {
    const auditData = {
      user_id: userId,
      action: "LOGIN_FAILED",
      ip_address: req.ip,
      user_agent: req.get("User-Agent"),
      success: false,
      details: JSON.stringify({
        email: email,
        reason: reason,
        timestamp: new Date().toISOString(),
      }),
    };

    // Record via storage-service (non-blocking)
    this.storageClient.logAuditEvent(auditData);

    if (userId) {
      this.storageClient.recordLoginAttempt(userId, req.ip, false);
    }
  }
}

export default new AuthenticationService();
