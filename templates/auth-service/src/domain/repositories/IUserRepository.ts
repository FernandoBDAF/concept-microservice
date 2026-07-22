import type {
  CreateUserDTO,
  PaginatedResult,
  PaginationParams,
  UpdateUserDTO,
} from "../../types/index.js";
import type { UserEntity } from "../entities/User.js";

export interface IUserRepository {
  create(data: CreateUserDTO): Promise<UserEntity>;
  findById(id: string): Promise<UserEntity | null>;
  findByEmail(email: string): Promise<UserEntity | null>;
  update(id: string, data: UpdateUserDTO): Promise<UserEntity | null>;
  delete(id: string): Promise<boolean>;
  list(params: PaginationParams): Promise<PaginatedResult<UserEntity>>;
  recordLoginAttempt(id: string, success: boolean): Promise<UserEntity | null>;
  validatePassword(user: UserEntity, password: string): Promise<boolean>;
}

