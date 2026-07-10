import { beforeEach, describe, expect, it } from "vitest";
import { UserService } from "../UserService.js";
import { createMockUserRepository } from "../../../__tests__/mocks/userRepository.mock.js";
import { createMockUser } from "../../../__tests__/factories/user.factory.js";
import {
  ConflictError,
  NotFoundError,
  ValidationError,
} from "../../../utils/errors.js";

describe("UserService", () => {
  let userService: UserService;
  let mockUserRepository: ReturnType<typeof createMockUserRepository>;

  beforeEach(() => {
    mockUserRepository = createMockUserRepository();
    userService = new UserService(mockUserRepository);
  });

  it("creates user successfully", async () => {
    const mockUser = createMockUser();
    mockUserRepository.findByEmail.mockResolvedValue(null);
    mockUserRepository.create.mockResolvedValue(mockUser);

    const result = await userService.createUser({
      email: "test@example.com",
      password: "securePassword123",
      role: "user",
    });

    expect(result.email).toBe(mockUser.email);
  });

  it("throws ValidationError for short password", async () => {
    await expect(
      userService.createUser({
        email: "test@example.com",
        password: "short",
        role: "user",
      })
    ).rejects.toThrow(ValidationError);
  });

  it("throws ConflictError for duplicate email", async () => {
    const mockUser = createMockUser();
    mockUserRepository.findByEmail.mockResolvedValue(mockUser);

    await expect(
      userService.createUser({
        email: mockUser.email,
        password: "securePassword123",
        role: "user",
      })
    ).rejects.toThrow(ConflictError);
  });

  it("throws NotFoundError when user missing", async () => {
    mockUserRepository.findById.mockResolvedValue(null);

    await expect(userService.getUserById("missing")).rejects.toThrow(
      NotFoundError
    );
  });
});

