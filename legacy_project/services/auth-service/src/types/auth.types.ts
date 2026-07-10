import type { SafeUser } from "./user.types.js";

export interface TokenPair {
  accessToken: string;
  refreshToken: string;
  tokenType: "bearer";
  expiresIn: number;
}

export interface TokenPayload {
  userId: string;
  email: string;
  role: string;
  tokenType: "ACCESS_TOKEN" | "REFRESH_TOKEN";
  jti: string;
  iat: number;
  exp: number;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  user: SafeUser;
  tokens: TokenPair;
}

export interface TokenValidationResult {
  valid: boolean;
  user?: SafeUser;
  error?: string;
}

export interface RefreshTokenRequest {
  refreshToken: string;
}

export interface AuthenticatedUser {
  id: string;
  email: string;
  role: string;
}

