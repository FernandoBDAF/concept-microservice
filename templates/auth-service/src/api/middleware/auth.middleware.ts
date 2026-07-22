import type { NextFunction, Request, Response } from "express";
import { ForbiddenError, UnauthorizedError } from "../../utils/errors.js";
import { tokenService } from "../../domain/services/TokenService.js";
import { userRepository } from "../../domain/repositories/UserRepository.js";

export const requiresAuth = (roles: string[] = []) => {
  return async (req: Request, _res: Response, next: NextFunction) => {
    try {
      const token = req.headers.authorization?.split(" ")[1];

      if (!token) {
        throw new UnauthorizedError("Authorization token required");
      }

      const decoded = tokenService.verifyAccessToken(token);
      const user = await userRepository.findById(decoded.userId);

      if (!user?.isActive) {
        throw new UnauthorizedError("User not found or inactive");
      }

      if (roles.length > 0 && !roles.includes(user.role)) {
        throw new ForbiddenError("Access denied");
      }

      req.user = {
        id: user.id,
        email: user.email,
        role: user.role,
      };

      next();
    } catch (error) {
      next(error);
    }
  };
};

