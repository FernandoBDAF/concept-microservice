export type UserRole = "user" | "admin";

export interface User {
  id: string;
  email: string;
  hashedPassword: string;
  role: UserRole;
  isActive: boolean;
  failedAttempts: number;
  lockedUntil: Date | null;
  createdAt: Date;
  updatedAt: Date;
}

export interface SafeUser {
  id: string;
  email: string;
  role: UserRole;
  isActive: boolean;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateUserDTO {
  email: string;
  password: string;
  role?: UserRole;
}

export interface UpdateUserDTO {
  email?: string;
  password?: string;
  role?: UserRole;
  isActive?: boolean;
}

export interface UserFilters {
  role?: UserRole;
  isActive?: boolean;
}

export interface PaginationParams {
  page: number;
  pageSize: number;
}

export interface PaginatedResult<T> {
  data: T[];
  pagination: {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
  };
}

