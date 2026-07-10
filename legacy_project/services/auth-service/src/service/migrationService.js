import fs from "fs/promises";
import path from "path";
import { fileURLToPath } from "url";
import db from "./databaseService.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

class MigrationService {
  constructor() {
    this.migrationsPath = path.join(__dirname, "../../migrations");
  }

  async createMigrationsTable() {
    const query = `
      CREATE TABLE IF NOT EXISTS migrations (
        id SERIAL PRIMARY KEY,
        filename VARCHAR(255) UNIQUE NOT NULL,
        executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        checksum VARCHAR(64) NOT NULL
      );
    `;

    console.log("Creating migrations table...");
    await db.migrationQuery(query); // Use migrationQuery for extended timeout
    console.log("✅ Migrations table ready");
  }

  async getExecutedMigrations() {
    try {
      const result = await db.query(
        "SELECT filename FROM migrations ORDER BY id"
      );
      return result.rows.map((row) => row.filename);
    } catch (error) {
      // If migrations table doesn't exist, return empty array
      if (error.code === "42P01") {
        return [];
      }
      throw error;
    }
  }

  async getMigrationFiles() {
    try {
      const files = await fs.readdir(this.migrationsPath);
      return files.filter((file) => file.endsWith(".sql")).sort(); // Ensure consistent order
    } catch (error) {
      console.warn("Migrations directory not found, creating it...");
      await fs.mkdir(this.migrationsPath, { recursive: true });
      return [];
    }
  }

  async calculateChecksum(content) {
    const crypto = await import("crypto");
    return crypto.createHash("sha256").update(content).digest("hex");
  }

  async executeMigration(filename) {
    const filePath = path.join(this.migrationsPath, filename);
    const content = await fs.readFile(filePath, "utf8");
    const checksum = await this.calculateChecksum(content);

    console.log(`🔄 Executing migration: ${filename}`);

    await db.transaction(async (client) => {
      // Set extended timeout for migration transaction
      await client.query("SET statement_timeout = 120000"); // 2 minutes

      console.log(`Executing migration SQL for ${filename}...`);
      // Execute the migration SQL
      await client.query(content);
      console.log(`Migration SQL completed for ${filename}`);

      // Record the migration
      await client.query(
        "INSERT INTO migrations (filename, checksum) VALUES ($1, $2)",
        [filename, checksum]
      );
      console.log(`Migration recorded in database: ${filename}`);
    });

    console.log(`✅ Migration completed: ${filename}`);
  }

  async runMigrations() {
    console.log("🚀 Starting database migrations...");

    // Add a small delay to ensure database is fully ready
    console.log("⏳ Waiting for database to be fully ready...");
    await new Promise((resolve) => setTimeout(resolve, 5000)); // 5 second delay

    // Test database connectivity first
    console.log("🔍 Testing database connectivity...");
    const isHealthy = await db.healthCheck();
    if (!isHealthy) {
      throw new Error("Database health check failed before migrations");
    }
    console.log("✅ Database connectivity confirmed");

    // Ensure migrations table exists
    await this.createMigrationsTable();

    // Get executed and available migrations
    const executedMigrations = await this.getExecutedMigrations();
    const availableMigrations = await this.getMigrationFiles();

    // Find pending migrations
    const pendingMigrations = availableMigrations.filter(
      (migration) => !executedMigrations.includes(migration)
    );

    if (pendingMigrations.length === 0) {
      console.log("✅ No pending migrations");
      return;
    }

    console.log(`📋 Found ${pendingMigrations.length} pending migrations:`);
    pendingMigrations.forEach((migration) => console.log(`   - ${migration}`));

    // Execute pending migrations
    for (const migration of pendingMigrations) {
      await this.executeMigration(migration);
    }

    console.log("🎉 All migrations completed successfully!");
  }

  async getMigrationStatus() {
    await this.createMigrationsTable();

    const executedMigrations = await this.getExecutedMigrations();
    const availableMigrations = await this.getMigrationFiles();
    const pendingMigrations = availableMigrations.filter(
      (migration) => !executedMigrations.includes(migration)
    );

    return {
      total: availableMigrations.length,
      executed: executedMigrations.length,
      pending: pendingMigrations.length,
      executedMigrations,
      pendingMigrations,
    };
  }

  async rollbackLastMigration() {
    console.log("⚠️  Rollback functionality not implemented yet");
    console.log("   Manual rollback required for now");

    const status = await this.getMigrationStatus();
    if (status.executed === 0) {
      console.log("❌ No migrations to rollback");
      return;
    }

    console.log(
      `📋 Last executed migration: ${
        status.executedMigrations[status.executedMigrations.length - 1]
      }`
    );
    console.log("   Please create a rollback migration manually");
  }
}

export default new MigrationService();
