import type { Request, Response } from "express";
import { AuthService } from "../../domain/services/AuthService.js";
import { tokenService } from "../../domain/services/TokenService.js";
import { userRepository } from "../../domain/repositories/UserRepository.js";

const authService = new AuthService(userRepository, tokenService);

export class AuthController {
  static async login(req: Request, res: Response): Promise<void> {
    const { email, password } = req.body;
    const result = await authService.login(
      { email, password },
      {
        ip: req.ip,
        userAgent: req.get("User-Agent") ?? undefined,
      }
    );

    res.json({
      status: "success",
      message: "Authentication successful",
      data: {
        access_token: result.tokens.accessToken,
        refresh_token: result.tokens.refreshToken,
        token_type: result.tokens.tokenType,
        expires_in: result.tokens.expiresIn,
        user: result.user,
      },
    });
  }

  static async validateToken(req: Request, res: Response): Promise<void> {
    const token =
      req.headers.authorization?.split(" ")[1] ?? req.body.token ?? "";

    const validation = await authService.validateToken(token);

    if (validation.valid) {
      res.json({
        status: "success",
        message: "Token is valid",
        data: {
          valid: true,
          user: validation.user,
        },
      });
      return;
    }

    res.status(401).json({
      status: "error",
      message: "Invalid token",
      data: {
        valid: false,
      },
    });
  }

  static async refreshToken(req: Request, res: Response): Promise<void> {
    const { refresh_token } = req.body;
    const tokens = await authService.refreshTokens(refresh_token);

    res.json({
      status: "success",
      message: "Token refreshed successfully",
      data: {
        access_token: tokens.accessToken,
        refresh_token: tokens.refreshToken,
        token_type: tokens.tokenType,
        expires_in: tokens.expiresIn,
      },
    });
  }

  static async logout(req: Request, res: Response): Promise<void> {
    const token = req.headers.authorization?.split(" ")[1];
    if (token) {
      await authService.logout(token);
    }

    res.json({
      status: "success",
      message: "Logged out successfully",
      data: null,
    });
  }
}

