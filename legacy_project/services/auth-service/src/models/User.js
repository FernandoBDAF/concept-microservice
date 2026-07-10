class User {
  constructor(
    id,
    email,
    hashedPassword,
    salt,
    role,
    isActive,
    createdAt,
    updatedAt
  ) {
    this.id = id;
    this.email = email;
    this.hashedPassword = hashedPassword;
    this.salt = salt;
    this.role = role || "user";
    this.isActive = isActive !== undefined ? isActive : true;
    this.createdAt = createdAt || new Date();
    this.updatedAt = updatedAt || new Date();
  }

  toJSON() {
    return {
      id: this.id,
      email: this.email,
      hashedPassword: this.hashedPassword,
      salt: this.salt,
      role: this.role,
      isActive: this.isActive,
      createdAt: this.createdAt,
      updatedAt: this.updatedAt,
    };
  }

  // Exclude sensitive data when converting to JSON
  toSafeJSON() {
    return {
      id: this.id,
      email: this.email,
      role: this.role,
      isActive: this.isActive,
      createdAt: this.createdAt,
      updatedAt: this.updatedAt,
    };
  }

  // Validate user data
  static validate(userData) {
    const errors = [];

    if (!userData.email) {
      errors.push("Email is required");
    } else if (!userData.email.match(/^[^\s@]+@[^\s@]+\.[^\s@]+$/)) {
      errors.push("Invalid email format");
    }

    if (!userData.password && !userData.hashedPassword) {
      errors.push("Password is required");
    }

    if (userData.role && !["user", "admin"].includes(userData.role)) {
      errors.push("Invalid role");
    }

    return errors;
  }
}

export default User;
