import type {
  CreateUserDTO,
  PaginatedResult,
  PaginationParams,
  SafeUser,
  UpdateUserDTO,
} from "../../types/index.js";
import type { IUserRepository } from "../repositories/IUserRepository.js";
import {
  ConflictError,
  NotFoundError,
  ValidationError,
} from "../../utils/errors.js";
import { config } from "../../config/index.js";
import { logger } from "../../infrastructure/logging/logger.js";

export class UserService {
  private readonly log = logger.child({ service: "UserService" });

  constructor(private readonly userRepository: IUserRepository) {}

  async createUser(data: CreateUserDTO): Promise<SafeUser> {
    if (data.password.length < config.security.passwordMinLength) {
      throw new ValidationError(
        `Password must be at least ${String(
          config.security.passwordMinLength
        )} characters`
      );
    }

    const existing = await this.userRepository.findByEmail(data.email);
    if (existing) {
      throw new ConflictError("User with this email already exists");
    }

    const user = await this.userRepository.create(data);
    this.log.info({ userId: user.id, email: user.email }, "User created");
    return user.toSafeUser();
  }

  async getUserById(id: string): Promise<SafeUser> {
    const user = await this.userRepository.findById(id);
    if (!user) {
      throw new NotFoundError("User");
    }
    return user.toSafeUser();
  }

  async getUserByEmail(email: string): Promise<SafeUser> {
    const user = await this.userRepository.findByEmail(email);
    if (!user) {
      throw new NotFoundError("User");
    }
    return user.toSafeUser();
  }

  async updateUser(id: string, data: UpdateUserDTO): Promise<SafeUser> {
    if (
      data.password &&
      data.password.length < config.security.passwordMinLength
    ) {
      throw new ValidationError(
        `Password must be at least ${String(
          config.security.passwordMinLength
        )} characters`
      );
    }

    if (data.email) {
      const existing = await this.userRepository.findByEmail(data.email);
      if (existing && existing.id !== id) {
        throw new ConflictError("User with this email already exists");
      }
    }

    const user = await this.userRepository.update(id, data);
    if (!user) {
      throw new NotFoundError("User");
    }

    this.log.info({ userId: id }, "User updated");
    return user.toSafeUser();
  }

  async deleteUser(id: string): Promise<void> {
    const deleted = await this.userRepository.delete(id);
    if (!deleted) {
      throw new NotFoundError("User");
    }
    this.log.info({ userId: id }, "User deleted");
  }

  async listUsers(
    params: PaginationParams
  ): Promise<PaginatedResult<SafeUser>> {
    const result = await this.userRepository.list(params);
    return {
      data: result.data.map((user) => user.toSafeUser()),
      pagination: result.pagination,
    };
  }

  async deactivateUser(id: string): Promise<SafeUser> {
    return this.updateUser(id, { isActive: false });
  }

  async activateUser(id: string): Promise<SafeUser> {
    return this.updateUser(id, { isActive: true });
  }
}

