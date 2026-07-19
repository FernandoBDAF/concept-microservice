import { v4 as uuidv4 } from "uuid";
import type { CreateUserDTO, SafeUser, User } from "../../types/index.js";
import { UserEntity } from "../../domain/entities/User.js";

export const createMockUser = (overrides: Partial<User> = {}): UserEntity => {
  const defaults: User = {
    id: uuidv4(),
    email: `test-${String(Date.now())}@example.com`,
    hashedPassword: "hashed_password",
    role: "user",
    isActive: true,
    failedAttempts: 0,
    lockedUntil: null,
    createdAt: new Date(),
    updatedAt: new Date(),
  };

  return new UserEntity(
    overrides.id ?? defaults.id,
    overrides.email ?? defaults.email,
    overrides.hashedPassword ?? defaults.hashedPassword,
    overrides.role ?? defaults.role,
    overrides.isActive ?? defaults.isActive,
    overrides.failedAttempts ?? defaults.failedAttempts,
    overrides.lockedUntil ?? defaults.lockedUntil,
    overrides.createdAt ?? defaults.createdAt,
    overrides.updatedAt ?? defaults.updatedAt
  );
};

export const createMockCreateUserDTO = (
  overrides: Partial<CreateUserDTO> = {}
): CreateUserDTO => ({
  email: `test-${String(Date.now())}@example.com`,
  password: "securePassword123",
  role: "user",
  ...overrides,
});

export const createMockSafeUser = (
  overrides: Partial<SafeUser> = {}
): SafeUser => ({
  id: uuidv4(),
  email: `test-${String(Date.now())}@example.com`,
  role: "user",
  isActive: true,
  createdAt: new Date(),
  updatedAt: new Date(),
  ...overrides,
});

