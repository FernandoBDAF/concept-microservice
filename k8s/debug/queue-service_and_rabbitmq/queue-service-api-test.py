#!/usr/bin/env python3
"""
Queue Service API Test Suite

This script implements a comprehensive test suite for the Queue Service API.
It tests the following functionality:
1. Service health check
2. Queue operations (publish and status)
3. Prometheus metrics exposure

The test suite is designed to run in a Kubernetes environment and uses the
queue-service's internal DNS name for communication.
"""

import os
import json
import time
import uuid
import requests
import logging
from datetime import datetime
from typing import List, Dict, Any, Optional, Tuple

# Configure logging with timestamp and log level
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Service configuration
# Using port 80 as configured in the Kubernetes service
BASE_URL = "http://queue-service:80"

# Retry configuration for service availability check
MAX_RETRIES = 30  # Maximum number of retry attempts
RETRY_DELAY = 5   # Delay between retries in seconds

# Message validation constants
REQUIRED_MESSAGE_FIELDS = ["id", "type", "timestamp", "payload", "priority"]
REQUIRED_HEADER_FIELDS = ["correlation_id"]
TEST_TYPES = ["profile_update", "cache_invalidation", "background_job"]
MIN_PRIORITY = 0  # Minimum allowed message priority
MAX_PRIORITY = 9  # Maximum allowed message priority

def wait_for_service():
    """
    Wait for the queue service to become available.
    
    This function attempts to connect to the service's health endpoint
    multiple times with a delay between attempts. It's used to ensure
    the service is ready before running the actual tests.
    
    Returns:
        bool: True if service becomes available, False otherwise
    """
    for i in range(MAX_RETRIES):
        try:
            response = requests.get(f"{BASE_URL}/health")
            if response.status_code == 200:
                logger.info("✅ Service is available")
                return True
        except requests.exceptions.RequestException as e:
            if i < MAX_RETRIES - 1:
                logger.info(f"Waiting for service to be ready... (attempt {i+1}/{MAX_RETRIES})")
                time.sleep(RETRY_DELAY)
            else:
                logger.error(f"❌ Service failed to become available after {MAX_RETRIES} attempts")
                return False
    return False

def test_health_check():
    """
    Test the health check endpoint of the queue service.
    
    This test verifies that:
    1. The health endpoint is accessible
    2. It returns a 200 status code
    3. The service is reporting as healthy
    
    Returns:
        bool: True if health check passes, False otherwise
    """
    logger.info("\n=== Testing Health Check ===")
    try:
        response = requests.get(f"{BASE_URL}/health")
        if response.status_code == 200:
            logger.info("✅ Health check passed")
            return True
        else:
            logger.error(f"❌ Health check failed with status code: {response.status_code}")
            return False
    except requests.exceptions.RequestException as e:
        logger.error(f"❌ Health check failed: {str(e)}")
        return False

def validate_message_structure(message: Dict[str, Any]) -> Tuple[bool, Optional[str]]:
    """
    Validate the structure and content of a queue message.
    
    This function checks:
    1. All required fields are present
    2. Message type is valid
    3. Payload is a valid object
    4. Headers contain required fields
    5. Priority is within valid range
    
    Args:
        message (Dict[str, Any]): The message to validate
        
    Returns:
        Tuple[bool, Optional[str]]: (is_valid, error_message)
    """
    # Check required fields
    for field in REQUIRED_MESSAGE_FIELDS:
        if field not in message:
            return False, f"Missing required field: {field}"

    # Validate message type
    if message["type"] not in TEST_TYPES:
        return False, f"Invalid message type: {message['type']}"

    # Validate payload
    if not isinstance(message["payload"], dict):
        return False, "Payload must be an object"

    # Validate headers if present
    if "headers" in message:
        headers = message["headers"]
        if not isinstance(headers, dict):
            return False, "Headers must be an object"

        # Check required header fields
        for field in REQUIRED_HEADER_FIELDS:
            if field not in headers:
                return False, f"Missing required header field: {field}"

        # Validate priority
        if "priority" in headers:
            try:
                priority = int(headers["priority"])
                if not MIN_PRIORITY <= priority <= MAX_PRIORITY:
                    return False, f"Priority must be between {MIN_PRIORITY} and {MAX_PRIORITY}"
            except ValueError:
                return False, "Priority must be an integer"

    return True, None

def test_queue_operations():
    """
    Test queue operations including message publishing and status retrieval.
    
    This test:
    1. Creates a test message with all required fields
    2. Validates the message structure
    3. Publishes the message to the queue
    4. Retrieves and validates the message status
    
    Returns:
        bool: True if all operations succeed, False otherwise
    """
    logger.info("\n=== Testing Queue Operations ===")
    
    # Create test message with all required fields
    test_message = {
        "id": str(uuid.uuid4()),
        "type": "profile_update",
        "timestamp": datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "correlation_id": str(uuid.uuid4()),
        "payload": {
            "user_id": "123",
            "changes": {
                "name": "John Doe"
            }
        },
        "headers": {
            "correlation_id": str(uuid.uuid4()),
            "priority": "1"
        },
        "priority": 1
    }
    
    # Validate message structure before sending
    is_valid, error_msg = validate_message_structure(test_message)
    if not is_valid:
        logger.error(f"❌ Invalid message structure: {error_msg}")
        return False
    
    # Test publish message
    logger.info("Testing publish message operation...")
    try:
        response = requests.post(
            f"{BASE_URL}/api/v1/queue/messages",
            json=test_message
        )
        if response.status_code == 202:
            data = response.json()
            if "message_id" in data:
                message_id = data["message_id"]
                logger.info(f"✅ Message published successfully with ID: {message_id}")
                
                # Test get message status
                logger.info("Testing get message status...")
                status_response = requests.get(f"{BASE_URL}/api/v1/queue/status/{message_id}")
                if status_response.status_code == 200:
                    status_data = status_response.json()
                    if "status" in status_data and "timestamp" in status_data:
                        logger.info(f"✅ Message status retrieved: {status_data['status']}")
                        return True
                    else:
                        logger.error("❌ Message status response missing required fields")
                        return False
                else:
                    logger.error(f"❌ Get message status failed with status code: {status_response.status_code}")
                    return False
            else:
                logger.error("❌ Publish response missing message_id")
                return False
        else:
            logger.error(f"❌ Publish message failed with status code: {response.status_code}")
            return False
    except requests.exceptions.RequestException as e:
        logger.error(f"❌ Queue operation failed: {str(e)}")
        return False

def test_queue_metrics():
    """
    Test the Prometheus metrics endpoint.
    
    This test verifies that:
    1. The metrics endpoint is accessible
    2. All required metrics are present
    3. The metrics are in the correct format
    
    Required metrics:
    - queue_messages_total
    - queue_processing_duration_seconds
    - queue_size
    - queue_errors_total
    
    Returns:
        bool: True if all metrics are present, False otherwise
    """
    logger.info("\n=== Testing Queue Metrics ===")
    try:
        response = requests.get(f"{BASE_URL}/metrics")
        if response.status_code == 200:
            # Check for required Prometheus metrics
            metrics_text = response.text
            required_metrics = [
                "queue_messages_total",
                "queue_processing_duration_seconds",
                "queue_size",
                "queue_errors_total"
            ]
            
            missing_metrics = [metric for metric in required_metrics if metric not in metrics_text]
            if not missing_metrics:
                logger.info("✅ Queue metrics endpoint working")
                return True
            else:
                logger.error(f"❌ Queue metrics missing required metrics: {', '.join(missing_metrics)}")
                return False
        else:
            logger.error(f"❌ Queue metrics failed with status code: {response.status_code}")
            return False
    except requests.exceptions.RequestException as e:
        logger.error(f"❌ Queue metrics failed: {str(e)}")
        return False

def main():
    """
    Main test function that orchestrates all test cases.
    
    The test sequence:
    1. Wait for service to be available
    2. Run health check test
    3. Run queue operations test
    4. Run metrics test
    
    Exits with status code 1 if any test fails.
    """
    logger.info("Starting Queue Service API tests...")
    
    # Wait for service to be ready
    if not wait_for_service():
        logger.error("❌ Health check failed, aborting tests")
        return
    
    # Run tests in sequence
    tests = [
        ("Health Check", test_health_check),
        ("Queue Operations", test_queue_operations),
        ("Queue Metrics", test_queue_metrics)
    ]
    
    all_passed = True
    for test_name, test_func in tests:
        if not test_func():
            all_passed = False
            logger.error(f"❌ {test_name} test failed")
    
    if all_passed:
        logger.info("\n✅ All tests passed successfully!")
    else:
        logger.error("\n❌ Some tests failed")
        exit(1)

if __name__ == "__main__":
    main() 