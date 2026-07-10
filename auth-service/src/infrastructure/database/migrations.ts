import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import crypto from "node:crypto";
import { db } from "./connection.js";
import { logger } from "../logging/logger.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

class MigrationService {
  private migrationsPath = path.join(__dirname, "../../../migrations");

  async createMigrationsTable(): Promise<void> {
    const query = `
      CREATE TABLE IF NOT EXISTS migrations (
        id SERIAL PRIMARY KEY,
        filename VARCHAR(255) UNIQUE NOT NULL,
        executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        checksum VARCHAR(64) NOT NULL
      );
    `;
    await db.query(query);
  }

  async getExecutedMigrations(): Promise<string[]> {
    try {
      const result = await db.query("SELECT filename FROM migrations ORDER BY id");
      return result.rows.map((row) => row.filename as string);
    } catch (error) {
      const err = error as { code?: string };
      if (err.code === "42P01") {
        return [];
      }
      throw error;
    }
  }

  async getMigrationFiles(): Promise<string[]> {
    try {
      const files = await fs.readdir(this.migrationsPath);
      return files.filter((file) => file.endsWith(".sql")).sort();
    } catch {
      await fs.mkdir(this.migrationsPath, { recursive: true });
      return [];
    }
  }

  calculateChecksum(content: string): string {
    return crypto.createHash("sha256").update(content).digest("hex");
  }

  async executeMigration(filename: string): Promise<void> {
    const filePath = path.join(this.migrationsPath, filename);
    const content = await fs.readFile(filePath, "utf8");
    const checksum = this.calculateChecksum(content);

    logger.info({ filename }, "Executing migration");

    await db.transaction(async (client) => {
      await client.query(content);
      await client.query(
        "INSERT INTO migrations (filename, checksum) VALUES ($1, $2)",
        [filename, checksum]
      );
    });
  }

  async runMigrations(): Promise<void> {
    logger.info("Starting database migrations");

    await this.createMigrationsTable();

    const executedMigrations = await this.getExecutedMigrations();
    const availableMigrations = await this.getMigrationFiles();
    const pendingMigrations = availableMigrations.filter(
      (migration) => !executedMigrations.includes(migration)
    );

    if (pendingMigrations.length === 0) {
      logger.info("No pending migrations");
      return;
    }

    for (const migration of pendingMigrations) {
      await this.executeMigration(migration);
    }

    logger.info("All migrations completed successfully");
  }
}

export const migrationService = new MigrationService();

