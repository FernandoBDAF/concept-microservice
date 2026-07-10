import userRepository from "../repository/UserRepository.js";
import User from "../models/User.js";

class UserService {
  constructor() {
    this.userRepository = userRepository;
  }

  async createUser(userData) {
    // Validate user data
    const validationErrors = User.validate(userData);
    if (validationErrors.length > 0) {
      throw new Error(`Validation failed: ${validationErrors.join(", ")}`);
    }

    // Check if user already exists
    const existingUser = await this.userRepository.getUserByEmail(
      userData.email
    );
    if (existingUser) {
      throw new Error("User with this email already exists");
    }

    // Create user
    const user = await this.userRepository.createUser(userData);

    console.log(`User created: ${user.email} with ID: ${user.id}`);
    return user;
  }

  async getUserById(id) {
    if (!id) {
      throw new Error("User ID is required");
    }

    const user = await this.userRepository.getUserById(id);
    if (!user) {
      throw new Error("User not found");
    }

    return user;
  }

  async getUserByEmail(email) {
    if (!email) {
      throw new Error("Email is required");
    }

    const user = await this.userRepository.getUserByEmail(email);
    if (!user) {
      throw new Error("User not found");
    }

    return user;
  }

  async updateUser(id, userData) {
    if (!id) {
      throw new Error("User ID is required");
    }

    // Check if user exists
    const existingUser = await this.userRepository.getUserById(id);
    if (!existingUser) {
      throw new Error("User not found");
    }

    // Validate updated data
    if (userData.email || userData.password || userData.role) {
      const validationErrors = User.validate({
        email: userData.email || existingUser.email,
        password: userData.password || "dummy", // Skip password validation if not updating
        role: userData.role || existingUser.role,
      });

      if (
        validationErrors.length > 0 &&
        !(
          validationErrors.length === 1 &&
          validationErrors[0] === "Password is required"
        )
      ) {
        throw new Error(`Validation failed: ${validationErrors.join(", ")}`);
      }
    }

    // Check if email is being changed and already exists
    if (userData.email && userData.email !== existingUser.email) {
      const emailExists = await this.userRepository.getUserByEmail(
        userData.email
      );
      if (emailExists) {
        throw new Error("User with this email already exists");
      }
    }

    const updatedUser = await this.userRepository.updateUser(id, userData);

    console.log(
      `User updated: ${updatedUser.email} with ID: ${updatedUser.id}`
    );
    return updatedUser;
  }

  async deleteUser(id) {
    if (!id) {
      throw new Error("User ID is required");
    }

    // Check if user exists
    const existingUser = await this.userRepository.getUserById(id);
    if (!existingUser) {
      throw new Error("User not found");
    }

    const deleted = await this.userRepository.deleteUser(id);

    if (deleted) {
      console.log(
        `User deleted: ${existingUser.email} with ID: ${existingUser.id}`
      );
    }

    return deleted;
  }

  async listUsers(page = 1, pageSize = 10) {
    // Validate pagination parameters
    if (page < 1) page = 1;
    if (pageSize < 1 || pageSize > 100) pageSize = 10;

    const users = await this.userRepository.listUsers(page, pageSize);

    return {
      users,
      pagination: {
        page,
        pageSize,
        count: users.length,
      },
    };
  }

  async deactivateUser(id) {
    return this.updateUser(id, { isActive: false });
  }

  async activateUser(id) {
    return this.updateUser(id, { isActive: true });
  }

  async changeUserRole(id, role) {
    if (!["user", "admin"].includes(role)) {
      throw new Error('Invalid role. Must be "user" or "admin"');
    }

    return this.updateUser(id, { role });
  }
}

export default new UserService();
