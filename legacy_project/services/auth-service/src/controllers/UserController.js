import userService from "../service/userService.js";

class UserController {
  static async createUser(req, res) {
    try {
      const userData = req.body;
      const user = await userService.createUser(userData);

      res.status(201).json({
        status: "success",
        message: "User created successfully",
        data: {
          user: user.toSafeJSON(),
        },
      });
    } catch (error) {
      console.error("Create user error:", error);
      res.status(400).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async getUser(req, res) {
    try {
      const { id } = req.params;
      const user = await userService.getUserById(id);

      res.json({
        status: "success",
        message: "User retrieved successfully",
        data: {
          user: user.toSafeJSON(),
        },
      });
    } catch (error) {
      console.error("Get user error:", error);
      const statusCode = error.message === "User not found" ? 404 : 500;
      res.status(statusCode).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async getUserByEmail(req, res) {
    try {
      const { email } = req.params;
      const user = await userService.getUserByEmail(email);

      res.json({
        status: "success",
        message: "User retrieved successfully",
        data: {
          user: user.toSafeJSON(),
        },
      });
    } catch (error) {
      console.error("Get user by email error:", error);
      const statusCode = error.message === "User not found" ? 404 : 500;
      res.status(statusCode).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async updateUser(req, res) {
    try {
      const { id } = req.params;
      const userData = req.body;
      const user = await userService.updateUser(id, userData);

      res.json({
        status: "success",
        message: "User updated successfully",
        data: {
          user: user.toSafeJSON(),
        },
      });
    } catch (error) {
      console.error("Update user error:", error);
      const statusCode = error.message === "User not found" ? 404 : 400;
      res.status(statusCode).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async deleteUser(req, res) {
    try {
      const { id } = req.params;
      await userService.deleteUser(id);

      res.json({
        status: "success",
        message: "User deleted successfully",
        data: null,
      });
    } catch (error) {
      console.error("Delete user error:", error);
      const statusCode = error.message === "User not found" ? 404 : 500;
      res.status(statusCode).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async listUsers(req, res) {
    try {
      const page = parseInt(req.query.page) || 1;
      const pageSize = parseInt(req.query.pageSize) || 10;

      const result = await userService.listUsers(page, pageSize);

      res.json({
        status: "success",
        message: "Users retrieved successfully",
        data: {
          users: result.users.map((user) => user.toSafeJSON()),
          pagination: result.pagination,
        },
      });
    } catch (error) {
      console.error("List users error:", error);
      res.status(500).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async deactivateUser(req, res) {
    try {
      const { id } = req.params;
      const user = await userService.deactivateUser(id);

      res.json({
        status: "success",
        message: "User deactivated successfully",
        data: {
          user: user.toSafeJSON(),
        },
      });
    } catch (error) {
      console.error("Deactivate user error:", error);
      const statusCode = error.message === "User not found" ? 404 : 500;
      res.status(statusCode).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async activateUser(req, res) {
    try {
      const { id } = req.params;
      const user = await userService.activateUser(id);

      res.json({
        status: "success",
        message: "User activated successfully",
        data: {
          user: user.toSafeJSON(),
        },
      });
    } catch (error) {
      console.error("Activate user error:", error);
      const statusCode = error.message === "User not found" ? 404 : 500;
      res.status(statusCode).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }

  static async changeUserRole(req, res) {
    try {
      const { id } = req.params;
      const { role } = req.body;

      if (!role) {
        return res.status(400).json({
          status: "error",
          message: "Role is required",
          data: null,
        });
      }

      const user = await userService.changeUserRole(id, role);

      res.json({
        status: "success",
        message: "User role updated successfully",
        data: {
          user: user.toSafeJSON(),
        },
      });
    } catch (error) {
      console.error("Change user role error:", error);
      const statusCode = error.message === "User not found" ? 404 : 400;
      res.status(statusCode).json({
        status: "error",
        message: error.message,
        data: null,
      });
    }
  }
}

export default UserController;
