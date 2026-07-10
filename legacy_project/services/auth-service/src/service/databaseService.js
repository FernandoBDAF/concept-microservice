import pg from "pg";
import config from "../config/config.js";

class DatabaseService {
  constructor() {
    this.pool = new pg.Pool({
      host: process.env.DATABASE_HOST || "localhost",
      port: parseInt(process.env.DATABASE_PORT) || 5432,
      database: process.env.DATABASE_NAME || "auth_db",
      user: process.env.DATABASE_USER || "auth_user",
      password: process.env.DATABASE_PASSWORD || "development_password",
      max: parseInt(process.env.DATABASE_POOL_MAX) || 20,
      idleTimeoutMillis: 30000,
      connectionTimeoutMillis: 30000, // Increased from 10000 to 30000ms for migrations
      acquireTimeoutMillis: 30000, // Increased from 10000 to 30000ms
      createTimeoutMillis: 30000, // Increased from 10000 to 30000ms
      statement_timeout: 60000, // 60 second statement timeout for complex queries
      query_timeout: 60000, // 60 second query timeout
    });

    this._setupEventHandlers();
  }

  async query(text, params) {
    const client = await this.pool.connect();
    try {
      // Set statement timeout for this query
      await client.query("SET statement_timeout = 60000");
      const result = await client.query(text, params);
      return result;
    } finally {
      client.release();
    }
  }

  async transaction(callback) {
    const client = await this.pool.connect();
    try {
      // Set statement timeout for transactions
      await client.query("SET statement_timeout = 60000");
      await client.query("BEGIN");
      const result = await callback(client);
      await client.query("COMMIT");
      return result;
    } catch (error) {
      await client.query("ROLLBACK");
      throw error;
    } finally {
      client.release();
    }
  }

  async healthCheck() {
    try {
      await this.pool.query("SELECT 1");
      return true;
    } catch (error) {
      console.error("Database health check failed:", error);
      return false;
    }
  }

  // Special method for migrations with extended timeout
  async migrationQuery(text, params) {
    const client = await this.pool.connect();
    try {
      // Set extended timeout for migrations
      await client.query("SET statement_timeout = 120000"); // 2 minutes for migrations
      console.log("Executing migration query with extended timeout...");
      const result = await client.query(text, params);
      console.log("Migration query completed successfully");
      return result;
    } catch (error) {
      console.error("Migration query failed:", error);
      throw error;
    } finally {
      client.release();
    }
  }

  _setupEventHandlers() {
    this.pool.on("error", (err) => {
      console.error("Unexpected database error:", err);
    });

    this.pool.on("connect", () => {
      console.log("New database connection established");
    });

    this.pool.on("acquire", () => {
      // console.log("Database connection acquired from pool");
    });

    this.pool.on("remove", () => {
      console.log("Database connection removed from pool");
    });
  }

  async close() {
    await this.pool.end();
  }
}

export default new DatabaseService();
