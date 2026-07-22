export interface ApiResponse<T = unknown> {
  status: "success" | "error";
  message: string;
  data: T | null;
  requestId?: string;
  timestamp?: string;
}

export interface ApiError {
  status: "error";
  message: string;
  code: string;
  details?: Record<string, unknown>;
  stack?: string;
}

export interface HealthStatus {
  status: "healthy" | "degraded" | "unhealthy";
  service: string;
  version: string;
  timestamp: string;
  uptime: number;
  dependencies: Record<string, "healthy" | "unhealthy">;
}

export interface ReadinessStatus {
  status: "ready" | "not ready";
  timestamp: string;
  message: string;
}

export interface LivenessStatus {
  status: "alive";
  timestamp: string;
  uptime: number;
  memory: NodeJS.MemoryUsage;
}

