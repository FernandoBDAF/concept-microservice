import jwt from "jsonwebtoken";
import config from "../config/config.js";
import { v4 as uuidv4 } from "uuid";

class TokenService {
  async generateTokens(user) {
    const jti = uuidv4(); // Generate unique token ID

    const tokenPayload = {
      userId: user.id,
      email: user.email,
      firstName: user.first_name || user.firstName,
      lastName: user.last_name || user.lastName,
      role: user.role,
      jti: jti,
    };

    const accessToken = jwt.sign(
      {
        ...tokenPayload,
        tokenType: "ACCESS_TOKEN",
      },
      config.jwt.privateKeySecret,
      {
        expiresIn: config.jwt.accessTokenExpiry,
        algorithm: "HS256", // Use HMAC instead of RSA for simplicity
      }
    );

    const refreshToken = jwt.sign(
      {
        ...tokenPayload,
        tokenType: "REFRESH_TOKEN",
      },
      config.jwt.privateKeySecret,
      {
        expiresIn: config.jwt.refreshTokenExpiry,
        algorithm: "HS256",
      }
    );

    return {
      accessToken,
      refreshToken,
      jti,
    };
  }

  async verifyToken(token) {
    return jwt.verify(
      token,
      config.jwt.privateKeySecret,
      {
        algorithms: ["HS256"],
      }
    );
  }

  async verifyRefreshToken(refreshToken) {
    const decoded = jwt.verify(
      refreshToken,
      config.jwt.privateKeySecret,
      {
        algorithms: ["HS256"],
      }
    );

    if (decoded.tokenType !== "REFRESH_TOKEN") {
      throw new Error("Invalid refresh token type");
    }

    return decoded;
  }

  decodeToken(token) {
    return jwt.decode(token);
  }
}

export default new TokenService();
