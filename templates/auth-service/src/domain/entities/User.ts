import type { SafeUser, User, UserRole } from "../../types/index.js";

export class UserEntity implements User {
  constructor(
    public readonly id: string,
    public readonly email: string,
    public readonly hashedPassword: string,
    public readonly role: UserRole,
    public readonly isActive: boolean,
    public readonly failedAttempts: number,
    public readonly lockedUntil: Date | null,
    public readonly createdAt: Date,
    public readonly updatedAt: Date
  ) {}

  isLocked(): boolean {
    if (!this.lockedUntil) return false;
    return new Date() < this.lockedUntil;
  }

  canLogin(): boolean {
    return this.isActive && !this.isLocked();
  }

  toSafeUser(): SafeUser {
    return {
      id: this.id,
      email: this.email,
      role: this.role,
      isActive: this.isActive,
      createdAt: this.createdAt,
      updatedAt: this.updatedAt,
    };
  }

  static fromRow(row: UserRow): UserEntity {
    return new UserEntity(
      row.id,
      row.email,
      row.hashed_password,
      row.role as UserRole,
      row.is_active,
      row.failed_attempts,
      row.locked_until,
      row.created_at,
      row.updated_at
    );
  }
}

export interface UserRow {
  id: string;
  email: string;
  hashed_password: string;
  role: string;
  is_active: boolean;
  failed_attempts: number;
  locked_until: Date | null;
  created_at: Date;
  updated_at: Date;
}

