import {
  OpenAPIRegistry,
  OpenApiGeneratorV3,
  extendZodWithOpenApi,
} from "@asteasolutions/zod-to-openapi";
import { z } from "zod";

extendZodWithOpenApi(z);

export const registry = new OpenAPIRegistry();

registry.registerComponent("securitySchemes", "bearerAuth", {
  type: "http",
  scheme: "bearer",
  bearerFormat: "JWT",
});

const apiResponseSchema = z.object({
  status: z.enum(["success", "error"]),
  message: z.string(),
  data: z.unknown().nullable(),
});

registry.register("ApiResponse", apiResponseSchema);

export function generateOpenApiDocument() {
  const generator = new OpenApiGeneratorV3(registry.definitions);

  return generator.generateDocument({
    openapi: "3.0.3",
    info: {
      title: "Auth Service API",
      version: "2.0.0",
      description: "Modern authentication microservice with JWT-based authentication",
      contact: {
        name: "Fernando Barroso",
        email: "your-email@example.com",
      },
      license: {
        name: "MIT",
        url: "https://opensource.org/licenses/MIT",
      },
    },
    servers: [
      {
        url: "http://localhost:8080",
        description: "Development server",
      },
    ],
    tags: [
      { name: "Authentication", description: "Authentication endpoints" },
      { name: "Users", description: "User management endpoints" },
      { name: "Health", description: "Health check endpoints" },
    ],
  });
}

