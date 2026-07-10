"""
Graph Data API HTTP Server

Single entry point for all graph data endpoints.

Usage:
    python -m src.app.api.graph.server --port 8081
"""

# IMPORTANT: Load .env BEFORE any other imports that might use environment variables
import os

try:
    from dotenv import load_dotenv
    # Try multiple paths to find .env
    possible_paths = [
        os.path.join(os.path.dirname(__file__), '..', '..', '.env'),
        os.path.join(os.getcwd(), '.env'),
        '.env',
    ]
    for env_path in possible_paths:
        if os.path.exists(env_path):
            load_dotenv(env_path, override=True)
            print(f"✓ Loaded environment from: {os.path.abspath(env_path)}")
            break
    else:
        print("⚠ No .env file found")
except ImportError:
    print("⚠ python-dotenv not installed, using shell environment")

# Now import everything else
import argparse
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

from .router import handle_request
from .constants import DEFAULT_PORT, API_VERSION

# Use the project's logging library for consistent, Loki-compatible logging
from src.lib.logging import setup_logging, get_logger

# Setup logging with JSON format for Loki, writing to a log file
log_dir = os.path.join(os.path.dirname(__file__), '..', '..', 'logs')
os.makedirs(log_dir, exist_ok=True)
log_file = os.path.join(log_dir, 'graph_api.log')

# Initialize logging using the project's logging library
setup_logging(
    level=20,  # INFO
    log_file=log_file,
    json_format=True,  # Use JSON format for Loki
    silence_third_party=True,
    rotate_logs=True,
    max_bytes=10_000_000,  # 10MB
    backup_count=5,
)

logger = get_logger(__name__)


class GraphAPIHandler(BaseHTTPRequestHandler):
    """HTTP request handler for Graph Data API"""

    def do_GET(self):
        """Handle GET requests"""
        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/", "").lstrip("/")
        
        # Parse query parameters
        params = {k: v[0] for k, v in parse_qs(parsed.query).items()}

        result, status = handle_request("GET", path, params)
        self._send_response(result, status)

    def do_POST(self):
        """Handle POST requests"""
        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/", "").lstrip("/")
        
        # Parse query parameters
        params = {k: v[0] for k, v in parse_qs(parsed.query).items()}

        # Read request body
        body = None
        content_length = int(self.headers.get("Content-Length", 0))
        if content_length > 0:
            body_bytes = self.rfile.read(content_length)
            try:
                body = json.loads(body_bytes.decode("utf-8"))
            except json.JSONDecodeError:
                self._send_response({"error": "Invalid JSON body"}, 400)
                return

        result, status = handle_request("POST", path, params, body)
        self._send_response(result, status)

    def do_HEAD(self):
        """Handle HEAD requests (same as GET but no body)"""
        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/", "").lstrip("/")
        params = {k: v[0] for k, v in parse_qs(parsed.query).items()}

        result, status = handle_request("GET", path, params)
        self._send_headers(status, len(json.dumps(result, indent=2).encode()))

    def do_OPTIONS(self):
        """Handle CORS preflight requests"""
        self.send_response(200)
        self._add_cors_headers()
        self.end_headers()

    def _send_headers(self, status: int = 200, content_length: int = 0):
        """Send response headers only (for HEAD requests)"""
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(content_length))
        self._add_cors_headers()
        self.end_headers()

    def _send_response(self, data: dict, status: int = 200):
        """Send JSON response"""
        # Check if this is a raw format export (CSV, GraphML, etc.)
        if isinstance(data, dict) and data.get("format") in ["csv", "graphml", "gexf", "prometheus"]:
            content_type = {
                "csv": "text/csv",
                "graphml": "application/xml",
                "gexf": "application/xml",
                "prometheus": "text/plain",
            }.get(data["format"], "text/plain")
            
            self.send_response(status)
            self.send_header("Content-Type", content_type)
            self._add_cors_headers()
            self.end_headers()
            self.wfile.write(data.get("data", "").encode())
        else:
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self._add_cors_headers()
            self.end_headers()
            self.wfile.write(json.dumps(data, indent=2, default=str).encode())

    def _add_cors_headers(self):
        """Add CORS headers for browser access"""
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, HEAD, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")

    def log_message(self, format, *args):
        """Custom logging"""
        logger.info(f"{self.address_string()} - {format % args}")


def run_server(host: str = "0.0.0.0", port: int = DEFAULT_PORT):
    """Run the HTTP server"""
    server = HTTPServer((host, port), GraphAPIHandler)
    
    logger.info("=" * 60)
    logger.info(f"Graph Data API Server v{API_VERSION}")
    logger.info("=" * 60)
    logger.info(f"Running on http://{host}:{port}")
    logger.info("")
    logger.info("Available Endpoints:")
    logger.info("  GET  /api/health")
    logger.info("  GET  /api/entities/search")
    logger.info("  GET  /api/entities/{entity_id}")
    logger.info("  GET  /api/communities/search")
    logger.info("  GET  /api/communities/{community_id}")
    logger.info("  GET  /api/communities/levels")
    logger.info("  GET  /api/relationships/search")
    logger.info("  GET  /api/ego/network/{entity_id}")
    logger.info("  GET  /api/export/{format}")
    logger.info("  GET  /api/statistics")
    logger.info("  GET  /api/metrics")
    logger.info("  GET  /api/metrics/quality")
    logger.info("  GET  /api/metrics/performance")
    logger.info("=" * 60)

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Server stopped")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Graph Data API Server")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to")
    parser.add_argument("--port", type=int, default=DEFAULT_PORT, help="Port to listen on")
    args = parser.parse_args()

    run_server(args.host, args.port)
