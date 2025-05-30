#!/bin/sh

# Start the consumer in the background
# /app/consumer &

# Wait a bit for the consumer to start
# sleep 5

# Start the publisher
/app/publisher

# Keep the container running
wait 