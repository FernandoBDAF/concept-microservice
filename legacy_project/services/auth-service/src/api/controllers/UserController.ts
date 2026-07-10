import type { Request, Response } from "express";
import { UserService } from "../../domain/services/UserService.js";
import { userRepository } from "../../domain/repositories/UserRepository.js";
import { UnauthorizedError } from "../../utils/errors.js";

const userService = new UserService(userRepository);

export class UserController {
  static async createUser(req: Request, res: Response): Promise<void> {
    const user = await userService.createUser(req.body);
    res.status(201).json({
      status: "success",
      message: "User created successfully",
      data: { user },
    });
  }

  static async getProfile(req: Request, res: Response): Promise<void> {
    if (!req.user?.id) {
      throw new UnauthorizedError("Authorization token required");
    }

    const user = await userService.getUserById(req.user.id);
    res.json({
      status: "success",
      message: "User profile retrieved",
      data: { user },
    });
  }

  static async getUser(req: Request, res: Response): Promise<void> {
    const { id } = req.params;
    const user = await userService.getUserById(id);
    res.json({
      status: "success",
      message: "User retrieved successfully",
      data: { user },
    });
  }

  static async getUserByEmail(req: Request, res: Response): Promise<void> {
    const { email } = req.params;
    const user = await userService.getUserByEmail(email);
    res.json({
      status: "success",
      message: "User retrieved successfully",
      data: { user },
    });
  }

  static async updateUser(req: Request, res: Response): Promise<void> {
    const { id } = req.params;
    const user = await userService.updateUser(id, req.body);
    res.json({
      status: "success",
      message: "User updated successfully",
      data: { user },
    });
  }

  static async deleteUser(req: Request, res: Response): Promise<void> {
    const { id } = req.params;
    await userService.deleteUser(id);
    res.json({
      status: "success",
      message: "User deleted successfully",
      data: null,
    });
  }

  static async listUsers(req: Request, res: Response): Promise<void> {
    const page = parseInt(req.query.page as string, 10) || 1;
    const pageSize = parseInt(req.query.pageSize as string, 10) || 10;

    const result = await userService.listUsers({ page, pageSize });
    res.json({
      status: "success",
      message: "Users retrieved successfully",
      data: {
        users: result.data,
        pagination: result.pagination,
      },
    });
  }

  static async deactivateUser(req: Request, res: Response): Promise<void> {
    const { id } = req.params;
    const user = await userService.deactivateUser(id);
    res.json({
      status: "success",
      message: "User deactivated successfully",
      data: { user },
    });
  }

  static async activateUser(req: Request, res: Response): Promise<void> {
    const { id } = req.params;
    const user = await userService.activateUser(id);
    res.json({
      status: "success",
      message: "User activated successfully",
      data: { user },
    });
  }

  static async changeUserRole(req: Request, res: Response): Promise<void> {
    const { id } = req.params;
    const { role } = req.body;
    const user = await userService.updateUser(id, { role });
    res.json({
      status: "success",
      message: "User role updated successfully",
      data: { user },
    });
  }
}

