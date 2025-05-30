#!/usr/bin/env python3
import os
import pika
import time

# RabbitMQ connection parameters from environment variables
RABBITMQ_USERNAME = os.environ.get('RABBITMQ_USERNAME')
RABBITMQ_PASSWORD = os.environ.get('RABBITMQ_PASSWORD')
RABBITMQ_VHOST = os.environ.get('RABBITMQ_VHOST', '/')
RABBITMQ_HOST = 'rabbitmq'
RABBITMQ_PORT = 5672

# Queue name for the test
QUEUE_NAME = 'test_queue'

# Create connection parameters
credentials = pika.PlainCredentials(RABBITMQ_USERNAME, RABBITMQ_PASSWORD)
parameters = pika.ConnectionParameters(
    host=RABBITMQ_HOST,
    port=RABBITMQ_PORT,
    virtual_host=RABBITMQ_VHOST,
    credentials=credentials
)

# Connect to RabbitMQ
print("Connecting to RabbitMQ...")
connection = pika.BlockingConnection(parameters)
channel = connection.channel()

# Declare a queue
print(f"Declaring queue: {QUEUE_NAME}")
channel.queue_declare(queue=QUEUE_NAME)

# Publish a test message
message = "Hello, RabbitMQ!"
print(f"Publishing message: {message}")
channel.basic_publish(exchange='', routing_key=QUEUE_NAME, body=message)

# Consume the message
print("Consuming message...")
def callback(ch, method, properties, body):
    print(f"Received message: {body.decode()}")
    ch.stop_consuming()

channel.basic_consume(queue=QUEUE_NAME, on_message_callback=callback, auto_ack=True)
channel.start_consuming()

# Close the connection
connection.close()
print("AMQP test completed successfully.") 