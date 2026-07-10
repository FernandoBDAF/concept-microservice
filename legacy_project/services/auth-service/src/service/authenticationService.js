import userRepository from "../repository/UserRepository.js";
import tokenService from "./tokenService.js";

class AuthenticationService {
  constructor() {
    this.userRepository = userRepository;
    this.tokenService = tokenService;
  }

  async authenticateUser(email, password, req) {
    const startTime = Date.now();

    try {
      console.log(`Authentication attempt for user: ${email}`);

      // 1. Get user from database
      const user = await this.userRepository.getUserByEmail(email);

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
      if (!user.isActive) {
        await this._recordFailedAttempt(
          user.id,
          email,
          req,
          "ACCOUNT_INACTIVE"
        );
        throw new Error("Account is inactive");
      }

      // 4. Validate password
      const isValid = await this.userRepository.validatePassword(
        user,
        password
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

      // 6. Record successful login
      await this.userRepository.recordLoginAttempt(user.id, true);

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
          user: user.toSafeJSON(),
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
      // Verify JWT token
      const decoded = await this.tokenService.verifyToken(token);

      // Get user data
      const user = await this.userRepository.getUserById(decoded.userId);

      if (!user || !user.isActive) {
        return {
          valid: false,
          error: "User not found or inactive",
        };
      }

      return {
        valid: true,
        user: user.toSafeJSON(),
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

      // 2. Get user data
      const user = await this.userRepository.getUserById(decoded.userId);

      if (!user || !user.isActive) {
        throw new Error("User not found or inactive");
      }

      // 3. Generate new tokens
      const tokens = await this.tokenService.generateTokens(user);

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

      // Record logout in user's login history
      await this.userRepository.recordLoginAttempt(decoded.userId, true);

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
    if (userId) {
      await this.userRepository.recordLoginAttempt(userId, false);
    }

    console.error("Authentication failed:", {
      userId,
      email,
      reason,
      ip: req.ip,
      userAgent: req.get("User-Agent"),
      timestamp: new Date().toISOString(),
    });
  }
}

export default new AuthenticationService();
