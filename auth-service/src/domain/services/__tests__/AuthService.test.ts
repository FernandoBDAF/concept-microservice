import { describe, expect, it, beforeEach } from "vitest";
import { AuthService } from "../AuthService.js";
import { TokenService } from "../TokenService.js";
import { createMockUserRepository } from "../../../__tests__/mocks/userRepository.mock.js";
import {
  createMockSessionRepository,
  createMockSession,
} from "../../../__tests__/mocks/sessionRepository.mock.js";
import { createMockAuditLogRepository } from "../../../__tests__/mocks/auditLogRepository.mock.js";
import { createMockUser } from "../../../__tests__/factories/user.factory.js";
import { AccountLockedError, UnauthorizedError } from "../../../utils/errors.js";

describe("AuthService", () => {
  let authService: AuthService;
  let mockUserRepository: ReturnType<typeof createMockUserRepository>;
  let mockSessionRepository: ReturnType<typeof createMockSessionRepository>;
  let mockAuditLogRepository: ReturnType<typeof createMockAuditLogRepository>;
  let tokenService: TokenService;

  beforeEach(() => {
    mockUserRepository = createMockUserRepository();
    mockSessionRepository = createMockSessionRepository();
    mockAuditLogRepository = createMockAuditLogRepository();
    tokenService = new TokenService();
    authService = new AuthService(
      mockUserRepository,
      tokenService,
      mockSessionRepository,
      mockAuditLogRepository
    );
  });

  describe("login", () => {
    it("logs in with valid credentials and opens a session", async () => {
      const mockUser = createMockUser({ email: "test@example.com" });
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);
      mockUserRepository.validatePassword.mockResolvedValue(true);
      mockUserRepository.recordLoginAttempt.mockResolvedValue(mockUser);

      const result = await authService.login({
        email: "test@example.com",
        password: "password123",
      });

      expect(result.user.email).toBe("test@example.com");
      expect(result.tokens.accessToken).toBeDefined();
      expect(result.tokens.refreshToken).toBeDefined();
      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockUserRepository.recordLoginAttempt).toHaveBeenCalledWith(
        mockUser.id,
        true
      );
      // A rotating refresh session is created for the issued refresh token.
      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockSessionRepository.create).toHaveBeenCalledOnce();
    });

    it("throws UnauthorizedError for unknown user", async () => {
      mockUserRepository.findByEmail.mockResolvedValue(null);

      await expect(
        authService.login({ email: "unknown@example.com", password: "password" })
      ).rejects.toThrow(UnauthorizedError);
    });

    it("throws UnauthorizedError for invalid password", async () => {
      const mockUser = createMockUser();
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);
      mockUserRepository.validatePassword.mockResolvedValue(false);

      await expect(
        authService.login({ email: mockUser.email, password: "wrong" })
      ).rejects.toThrow(UnauthorizedError);

      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockUserRepository.recordLoginAttempt).toHaveBeenCalledWith(
        mockUser.id,
        false
      );
    });

    it("throws AccountLockedError for locked account", async () => {
      const lockedUntil = new Date(Date.now() + 3600 * 1000);
      const mockUser = createMockUser({ lockedUntil });
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);

      await expect(
        authService.login({ email: mockUser.email, password: "password" })
      ).rejects.toThrow(AccountLockedError);
    });

    it("throws UnauthorizedError for inactive account", async () => {
      const mockUser = createMockUser({ isActive: false });
      mockUserRepository.findByEmail.mockResolvedValue(mockUser);

      await expect(
        authService.login({ email: mockUser.email, password: "password" })
      ).rejects.toThrow(UnauthorizedError);
    });
  });

  describe("validateToken", () => {
    it("returns valid result for valid token", async () => {
      const mockUser = createMockUser();
      mockUserRepository.findById.mockResolvedValue(mockUser);
      const tokens = tokenService.generateTokens(mockUser);

      const result = await authService.validateToken(tokens.accessToken);

      expect(result.valid).toBe(true);
      expect(result.user?.id).toBe(mockUser.id);
    });

    it("returns invalid result for inactive user", async () => {
      const mockUser = createMockUser({ isActive: false });
      mockUserRepository.findById.mockResolvedValue(mockUser);
      const tokens = tokenService.generateTokens(mockUser);

      const result = await authService.validateToken(tokens.accessToken);

      expect(result.valid).toBe(false);
    });
  });

  describe("refreshTokens", () => {
    it("rotates the session on a valid current refresh token", async () => {
      const mockUser = createMockUser();
      const tokens = tokenService.generateTokens(mockUser);
      const jti = tokenService.decodeToken(tokens.refreshToken)?.jti ?? "";

      mockSessionRepository.findByRefreshTokenId.mockResolvedValue(
        createMockSession({ refreshTokenId: jti, userId: mockUser.id })
      );
      mockUserRepository.findById.mockResolvedValue(mockUser);

      const rotated = await authService.refreshTokens(tokens.refreshToken);

      expect(rotated.accessToken).toBeDefined();
      expect(rotated.refreshToken).toBeDefined();
      // Rotation records the presented jti as the previous token.
      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockSessionRepository.rotate).toHaveBeenCalledOnce();
      const rotateArgs = mockSessionRepository.rotate.mock.calls[0];
      expect(rotateArgs?.[2]).toBe(jti); // previousTokenId === presented jti
      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockSessionRepository.revoke).not.toHaveBeenCalled();
    });

    it("detects reuse of a rotated-out token: revokes session + audits", async () => {
      const mockUser = createMockUser();
      const tokens = tokenService.generateTokens(mockUser);
      const jti = tokenService.decodeToken(tokens.refreshToken)?.jti ?? "";

      // Presented jti is no longer any session's CURRENT token...
      mockSessionRepository.findByRefreshTokenId.mockResolvedValue(null);
      // ...but it is the PREVIOUS token of a live session -> reuse.
      const session = createMockSession({
        previousTokenId: jti,
        userId: mockUser.id,
      });
      mockSessionRepository.findByPreviousTokenId.mockResolvedValue(session);

      await expect(
        authService.refreshTokens(tokens.refreshToken)
      ).rejects.toThrow(UnauthorizedError);

      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockSessionRepository.revoke).toHaveBeenCalledWith(session.id);
      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockAuditLogRepository.record).toHaveBeenCalledOnce();
      const auditArg = mockAuditLogRepository.record.mock.calls[0]?.[0];
      expect(auditArg?.action).toBe("REFRESH_TOKEN_REUSE");
      expect(auditArg?.success).toBe(false);
    });

    it("rejects a refresh token whose session was revoked (logout)", async () => {
      const mockUser = createMockUser();
      const tokens = tokenService.generateTokens(mockUser);
      const jti = tokenService.decodeToken(tokens.refreshToken)?.jti ?? "";

      mockSessionRepository.findByRefreshTokenId.mockResolvedValue(
        createMockSession({
          refreshTokenId: jti,
          userId: mockUser.id,
          revokedAt: new Date(),
        })
      );

      await expect(
        authService.refreshTokens(tokens.refreshToken)
      ).rejects.toThrow(UnauthorizedError);
      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockSessionRepository.rotate).not.toHaveBeenCalled();
    });
  });

  describe("logout", () => {
    it("revokes the active session for the presented token", async () => {
      const mockUser = createMockUser();
      const tokens = tokenService.generateTokens(mockUser);
      const jti = tokenService.decodeToken(tokens.accessToken)?.jti ?? "";
      const session = createMockSession({
        refreshTokenId: jti,
        userId: mockUser.id,
      });
      mockSessionRepository.findByRefreshTokenId.mockResolvedValue(session);

      await authService.logout(tokens.accessToken);

      // eslint-disable-next-line @typescript-eslint/unbound-method
      expect(mockSessionRepository.revoke).toHaveBeenCalledWith(session.id);
    });
  });
});
