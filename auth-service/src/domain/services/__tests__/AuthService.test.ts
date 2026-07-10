import { describe, expect, it, beforeEach } from "vitest";
import { AuthService } from "../AuthService.js";
import { TokenService } from "../TokenService.js";
import { createMockUserRepository } from "../../../__tests__/mocks/userRepository.mock.js";
import { createMockUser } from "../../../__tests__/factories/user.factory.js";
import { AccountLockedError, UnauthorizedError } from "../../../utils/errors.js";

describe("AuthService", () => {
  let authService: AuthService;
  let mockUserRepository: ReturnType<typeof createMockUserRepository>;
  let tokenService: TokenService;

  beforeEach(() => {
    mockUserRepository = createMockUserRepository();
    tokenService = new TokenService();
    authService = new AuthService(mockUserRepository, tokenService);
  });

  describe("login", () => {
    it("logs in with valid credentials", async () => {
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
});

