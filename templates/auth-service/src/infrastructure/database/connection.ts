import pg from "pg";
import { config } from "../../config/index.js";
import { logger } from "../logging/logger.js";
import { DatabaseError } from "../../utils/errors.js";

const { Pool } = pg;

export interface DatabaseClient {
  query<T extends pg.QueryResultRow = pg.QueryResultRow>(
    text: string,
    params?: unknown[]
  ): Promise<pg.QueryResult<T>>;
  release(): void;
}

class Database {
  private pool: pg.Pool;
  private isConnected = false;

  constructor() {
    this.pool = new Pool({
      host: config.database.host,
      port: config.database.port,
      database: config.database.database,
      user: config.database.user,
      password: config.database.password,
      max: config.database.max,
      idleTimeoutMillis: 30000,
      connectionTimeoutMillis: 10000,
      ssl: config.database.ssl ? { rejectUnauthorized: false } : false,
    });

    this.setupEventHandlers();
  }

  private setupEventHandlers(): void {
    this.pool.on("connect", () => {
      this.isConnected = true;
      logger.debug("New database connection established");
    });

    this.pool.on("error", (err) => {
      logger.error({ err }, "Unexpected database pool error");
    });

    this.pool.on("remove", () => {
      logger.debug("Database connection removed from pool");
    });
  }

  async query<T extends pg.QueryResultRow = pg.QueryResultRow>(
    text: string,
    params?: unknown[]
  ): Promise<pg.QueryResult<T>> {
    const start = Date.now();
    try {
      const result = await this.pool.query<T>(text, params);
      const duration = Date.now() - start;
      logger.debug(
        { query: text, duration, rows: result.rowCount },
        "Query executed"
      );
      return result;
    } catch (error) {
      logger.error({ err: error, query: text }, "Query failed");
      throw new DatabaseError("Database query failed", error as Error);
    }
  }

  async getClient(): Promise<DatabaseClient> {
    return this.pool.connect();
  }

  async transaction<T>(
    callback: (client: DatabaseClient) => Promise<T>
  ): Promise<T> {
    const client = await this.getClient();
    try {
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

  async healthCheck(): Promise<boolean> {
    try {
      await this.pool.query("SELECT 1");
      return true;
    } catch {
      return false;
    }
  }

  async close(): Promise<void> {
    await this.pool.end();
    this.isConnected = false;
    logger.info("Database pool closed");
  }

  get connected(): boolean {
    return this.isConnected;
  }
}

export const db = new Database();

