# Queue Flow Test Implementation

This directory contains a test implementation of the new queue flow structure using RabbitMQ. The implementation follows the guidelines from the restructuring plan and serves as a proof of concept.

## Structure

```
.
├── cmd/
│   ├── consumer/     # Consumer implementation
│   └── publisher/    # Publisher implementation
├── pkg/
│   └── rabbitmq/     # Shared RabbitMQ constants
├── Dockerfile        # Multi-stage build for both services
├── entrypoint.sh     # Container startup script
├── go.mod           # Go module definition
├── rabbitmq.yaml    # RabbitMQ StatefulSet configuration
└── test-job.yaml    # Test job definition
```

## Prerequisites

- Go 1.20 or later
- Docker
- Kubernetes cluster (kind)
- kubectl

## Building and Running

1. Build the Docker image:

```bash
docker build -t queue-test:latest .
```

2. Load the image into kind:

```bash
kind load docker-image queue-test:latest
```

3. Deploy RabbitMQ:

```bash
kubectl apply -f rabbitmq.yaml
```

4. Wait for RabbitMQ to be ready:

```bash
kubectl wait --for=condition=ready pod/rabbitmq-0
```

5. Run the test job:

```bash
kubectl apply -f test-job.yaml
```

6. Check the logs:

```bash
kubectl logs -f job/queue-test
```

## Expected Output

The test will:

1. Start the consumer service
2. Wait for it to initialize
3. Start the publisher service
4. Send 5 test messages
5. Show the consumer processing those messages

You should see logs like:

```
Consumer started, waiting for messages on queue: profile-processing
Published message: Test message 1
Received message: Test message 1
Published message: Test message 2
Received message: Test message 2
...
```

## Cleanup

To clean up the test resources:

```bash
kubectl delete -f test-job.yaml
kubectl delete -f rabbitmq.yaml
```

## Notes

- The test uses a single container running both publisher and consumer for simplicity
- In production, these would be separate services
- The test uses basic authentication; production should use proper secrets
- The test uses ephemeral storage; production should use persistent volumes
