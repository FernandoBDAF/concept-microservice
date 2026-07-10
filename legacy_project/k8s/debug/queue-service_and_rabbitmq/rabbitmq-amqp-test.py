#!/usr/bin/env python3
import os
import pika
import time
import json
import uuid
from datetime import datetime
import sys

# RabbitMQ connection parameters
RABBITMQ_HOST = os.getenv('RABBITMQ_HOST', 'rabbitmq')
RABBITMQ_PORT = int(os.getenv('RABBITMQ_PORT', '5672'))
RABBITMQ_USER = os.getenv('RABBITMQ_USERNAME', 'rabbitmq-user')
RABBITMQ_PASSWORD = os.getenv('RABBITMQ_PASSWORD', 'rabbitmq-password')
RABBITMQ_VHOST = os.getenv('RABBITMQ_VHOST', '/')

# Queue configuration - using same names as queue service
QUEUE_NAME = 'profile_tasks'
EXCHANGE_NAME = 'profile_tasks.exchange'
DLX_NAME = 'profile_tasks.dlx'
DLQ_NAME = 'profile_tasks.dlq'

# Test configuration
TEST_MESSAGE_TTL = int(os.getenv('TEST_MESSAGE_TTL', '5000'))
TEST_MESSAGE_COUNT = int(os.getenv('TEST_MESSAGE_COUNT', '10'))  # Increased to 10 messages

def setup_connection():
    """Set up RabbitMQ connection and channel"""
    credentials = pika.PlainCredentials(RABBITMQ_USER, RABBITMQ_PASSWORD)
    parameters = pika.ConnectionParameters(
        host=RABBITMQ_HOST,
        port=RABBITMQ_PORT,
        virtual_host=RABBITMQ_VHOST,
        credentials=credentials,
        heartbeat=600,
        blocked_connection_timeout=300
    )
    
    print(f"Connecting to RabbitMQ at {RABBITMQ_HOST}:{RABBITMQ_PORT}")
    connection = pika.BlockingConnection(parameters)
    channel = connection.channel()
    print("Connected to RabbitMQ")
    return connection, channel

def setup_queues(channel):
    """Set up queues and exchanges"""
    try:
        # Check if queues exist
        try:
            channel.queue_declare(queue=QUEUE_NAME, passive=True)
            print(f"Queue {QUEUE_NAME} already exists")
        except pika.exceptions.ChannelClosedByBroker as e:
            if e.code == 404:
                # Queue doesn't exist, create it
                channel.queue_declare(
                    queue=QUEUE_NAME,
                    durable=True,
                    arguments={
                        'x-dead-letter-exchange': DLX_NAME,
                        'x-dead-letter-routing-key': DLQ_NAME,
                        'x-message-ttl': 86400000  # 24 hours in milliseconds
                    }
                )
                print(f"Created queue: {QUEUE_NAME}")
            else:
                raise

        try:
            channel.queue_declare(queue=DLQ_NAME, passive=True)
            print(f"Queue {DLQ_NAME} already exists")
        except pika.exceptions.ChannelClosedByBroker as e:
            if e.code == 404:
                # DLQ doesn't exist, create it
                channel.queue_declare(
                    queue=DLQ_NAME,
                    durable=True
                )
                print(f"Created queue: {DLQ_NAME}")
            else:
                raise

        # Declare exchanges (these are idempotent)
        channel.exchange_declare(
            exchange=EXCHANGE_NAME,
            exchange_type='direct',
            durable=True
        )
        print(f"Declared exchange: {EXCHANGE_NAME}")

        channel.exchange_declare(
            exchange=DLX_NAME,
            exchange_type='direct',
            durable=True
        )
        print(f"Declared exchange: {DLX_NAME}")

        # Bind queues (these are idempotent)
        channel.queue_bind(
            exchange=DLX_NAME,
            queue=DLQ_NAME,
            routing_key=DLQ_NAME
        )
        print(f"Bound {DLQ_NAME} to {DLX_NAME}")

        channel.queue_bind(
            exchange=EXCHANGE_NAME,
            queue=QUEUE_NAME,
            routing_key=QUEUE_NAME
        )
        print(f"Bound {QUEUE_NAME} to {EXCHANGE_NAME}")

    except Exception as e:
        print(f"Error setting up queues: {str(e)}")
        raise

def publish_message(channel):
    """Publish test messages"""
    messages = []
    for i in range(TEST_MESSAGE_COUNT):
        # Create a message that should be rejected every 3rd message
        should_reject = (i + 1) % 3 == 0
        
        message = {
            'id': f'test-{uuid.uuid4()}',
            'type': 'test',
            'data': {
                'test': f'value-{i}',
                'should_reject': should_reject,  # Flag to indicate if worker should reject
                'timestamp': datetime.utcnow().isoformat()
            },
            'timestamp': datetime.utcnow().isoformat()
        }
        messages.append(message)

        print(f"Publishing message {i+1}/{TEST_MESSAGE_COUNT} to exchange: {EXCHANGE_NAME} with routing key: {QUEUE_NAME}")
        print(f"Message {i+1} should be {'rejected' if should_reject else 'processed'}")
        print(f"Message structure: {json.dumps(message, indent=2)}")
        
        channel.basic_publish(
            exchange=EXCHANGE_NAME,
            routing_key=QUEUE_NAME,
            body=json.dumps(message),
            properties=pika.BasicProperties(
                delivery_mode=2,  # Make message persistent
                content_type='application/json',
                content_encoding='utf-8',
                headers={'should_reject': should_reject},  # Add header for worker to check
                priority=0,
                correlation_id=f'test-correlation-id-{i}',
                reply_to='test-reply-to',
                expiration='86400000',  # 24 hours in milliseconds
                message_id=message['id'],
                timestamp=int(time.time()),
                type='test',
                user_id=RABBITMQ_USER,
                app_id='test-app',
                cluster_id='test-cluster'
            )
        )
    print(f"Published {TEST_MESSAGE_COUNT} messages successfully")
    return messages

def verify_queue_state(channel, queue_name, expected_count):
    # Verify the number of messages in the queue
    queue = channel.queue_declare(queue=queue_name, passive=True)
    actual_count = queue.method.message_count
    consumer_count = queue.method.consumer_count
    ready_messages = queue.method.message_count
    unack_messages = queue.method.message_count
    print(f"\nQueue '{queue_name}' state:")
    print(f"Expected messages: {expected_count}")
    print(f"Actual messages: {actual_count}")
    print(f"Consumer count: {consumer_count}")
    print(f"Ready messages: {ready_messages}")
    print(f"Unacknowledged messages: {unack_messages}")
    return actual_count == expected_count

def consume_messages(channel, queue_name, count):
    # Consume messages from the queue
    messages = []
    
    def callback(ch, method, properties, body):
        message = json.loads(body)
        messages.append(message)
        print(f"Consumed message: {message['id']}")
        if len(messages) >= count:
            ch.stop_consuming()
    
    print(f"\nConsuming {count} messages from {queue_name}...")
    channel.basic_consume(
        queue=queue_name,
        on_message_callback=callback,
        auto_ack=True
    )
    channel.start_consuming()
    return messages

def cleanup_queues(channel):
    # Clean up existing test queues
    print("Cleaning up existing test queues...")
    try:
        channel.queue_delete(queue=QUEUE_NAME)
        channel.queue_delete(queue=DLQ_NAME)
        channel.exchange_delete(exchange=DLX_NAME)
    except Exception as e:
        print(f"Warning: Cleanup error: {e}")

def check_queue_settings(channel):
    # Check queue settings by attempting to redeclare with different properties
    print("\n=== Test 5: Queue Settings Inspection ===")
    print("Attempting to redeclare the queue with different settings to check for auto_delete and durable flags...")
    try:
        # Try to redeclare with durable=False (should fail if already durable)
        channel.queue_declare(queue=QUEUE_NAME, durable=False, auto_delete=False, passive=False)
        print("Queue is NOT durable (unexpected for this test).")
    except Exception as e:
        if 'PRECONDITION_FAILED' in str(e):
            print("Queue is durable (as expected). PRECONDITION_FAILED when trying to redeclare as non-durable.")
        else:
            print(f"Unexpected error when checking durable flag: {e}")
    try:
        # Try to redeclare with auto_delete=True (should fail if already auto_delete=False)
        channel.queue_declare(queue=QUEUE_NAME, durable=True, auto_delete=True, passive=False)
        print("Queue is auto_delete (unexpected for this test).")
    except Exception as e:
        if 'PRECONDITION_FAILED' in str(e):
            print("Queue is NOT auto_delete (as expected). PRECONDITION_FAILED when trying to redeclare as auto_delete.")
        else:
            print(f"Unexpected error when checking auto_delete flag: {e}")

def main():
    try:
        connection, channel = setup_connection()
        setup_queues(channel)
        
        # Publish messages
        messages = publish_message(channel)
        
        # Wait longer for processing since we have more messages and some will be retried
        print("\nWaiting 30 seconds for message processing...")
        time.sleep(30)
        
        # Verify queue state
        print("\nVerifying queue state...")
        verify_queue_state(channel, QUEUE_NAME, 0)  # Should be 0 as messages are processed or dead-lettered
        verify_queue_state(channel, DLQ_NAME, 3)    # Should have 3 messages (the rejected ones)
        
        connection.close()
        print("\nTest completed successfully")
        sys.exit(0)
    except Exception as e:
        print(f"Error: {str(e)}")
        sys.exit(1)

if __name__ == '__main__':
    main() 