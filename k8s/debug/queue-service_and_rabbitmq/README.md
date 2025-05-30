# Debugging RabbitMQ Connectivity in Kubernetes

This README documents the step-by-step process and commands used to debug and verify connectivity to the RabbitMQ pod from within the Kubernetes cluster using a debug/test pod.

## Steps

### 1. Create a Debug/Test Pod

A simple Alpine-based pod is used to run network and connectivity tests. The pod is defined in `rabbitmq-test-pod.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: rabbitmq-test
  labels:
    app: debug
spec:
  containers:
    - name: rabbitmq-test
      image: alpine:latest
      command: ["/bin/sh", "-c", "apk add --no-cache curl && sleep 3600"]
      env:
        - name: RABBITMQ_USERNAME
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secrets
              key: RABBITMQ_USERNAME
        - name: RABBITMQ_PASSWORD
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secrets
              key: RABBITMQ_PASSWORD
        - name: RABBITMQ_VHOST
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secrets
              key: RABBITMQ_VHOST
```

Apply the pod manifest:

```sh
kubectl apply -f k8s/debug/rabbitmq-test-pod.yaml
```

### 2. Wait for the Pod to be Ready

```sh
kubectl wait --for=condition=Ready pod/rabbitmq-test --timeout=60s
```

### 3. Install Netcat in the Debug Pod

```sh
kubectl exec rabbitmq-test -- sh -c "apk add --no-cache netcat-openbsd"
```

### 4. Test RabbitMQ AMQP Port Connectivity

```sh
kubectl exec rabbitmq-test -- sh -c "nc -zv rabbitmq 5672 && echo 'AMQP port is accessible'"
```

If successful, you should see output indicating the port is open. If not, you may see `Connection refused` or a timeout.

### 5. Check RabbitMQ Pod Status

```sh
kubectl get pods -l app=rabbitmq
```

### 6. Check RabbitMQ Pod Logs (if needed)

```sh
kubectl logs -l app=rabbitmq
```

## RabbitMQ AMQP Test

### Overview

This section documents the steps taken to validate RabbitMQ AMQP connectivity, message publishing, and consumption using a dedicated test pod. The test pod runs a Python script that connects to RabbitMQ, declares a queue, publishes a test message, and consumes it, confirming that RabbitMQ is fully functional and accessible from within the Kubernetes cluster.

### Steps to Run the AMQP Test

1. **Create the AMQP Test Pod Manifest**

   - The pod manifest (`rabbitmq-amqp-test-pod.yaml`) is located in the `k8s/debug` directory.
   - It uses the image `rabbitmq-amqp-test:latest` and sources RabbitMQ credentials from a Kubernetes secret.

2. **Create the Python Script**

   - The script (`rabbitmq-amqp-test.py`) is located in the `k8s/debug` directory.
   - It uses the `pika` library to connect to RabbitMQ, declare a queue, publish a message, and consume it.

3. **Create the Dockerfile**

   - The Dockerfile (`Dockerfile.amqp-test`) is located in the `k8s/debug` directory.
   - It uses the base image `python:3.9-slim` and installs the `pika` library.

4. **Build the Docker Image**

   - Run the following command to build the Docker image:
     ```bash
     docker build -t rabbitmq-amqp-test:latest -f k8s/debug/Dockerfile.amqp-test k8s/debug
     ```

5. **Load the Docker Image into the Cluster**

   - Run the following command to load the Docker image into the kind cluster:
     ```bash
     kind load docker-image rabbitmq-amqp-test:latest
     ```

6. **Deploy the AMQP Test Pod**

   - Run the following command to deploy the AMQP test pod:
     ```bash
     kubectl apply -f k8s/debug/rabbitmq-amqp-test-pod.yaml
     ```

7. **Check the Pod Logs**
   - Run the following command to check the logs of the AMQP test pod:
     ```bash
     kubectl logs pod/rabbitmq-amqp-test
     ```
   - The logs should indicate successful connection, queue declaration, message publishing, and consumption.

### Logic Behind the Implementation

- **Pod Manifest**: The pod manifest uses a Kubernetes secret to source RabbitMQ credentials, ensuring secure access to RabbitMQ.
- **Python Script**: The script uses the `pika` library to interact with RabbitMQ, providing a simple and effective way to test AMQP connectivity.
- **Dockerfile**: The Dockerfile uses a lightweight base image and installs only the necessary dependencies, ensuring a minimal and efficient container.

### Functionalities Tested

- **AMQP Connectivity**: The test confirms that the AMQP port (5672) is accessible and that the RabbitMQ pod is running.
- **Queue Declaration**: The test declares a queue, confirming that RabbitMQ is operational and can manage queues.
- **Message Publishing**: The test publishes a message to the declared queue, confirming that RabbitMQ can handle message publishing.
- **Message Consumption**: The test consumes the published message, confirming that RabbitMQ can handle message consumption.
- **Message Persistence**: The test verifies that messages are persisted in the queue, ensuring durability.
- **TTL and DLQ**: The test checks that messages with a TTL are moved to a Dead Letter Queue (DLQ) after expiration.
- **Queue Settings Inspection**: The test attempts to redeclare the queue with different settings to verify its durability and auto-delete properties.

### Conclusion

The AMQP test confirms that RabbitMQ is fully functional and accessible from within the Kubernetes cluster. The test pod successfully connects to RabbitMQ, declares a queue, publishes a message, and consumes it, validating the RabbitMQ deployment.

## Notes

- Ensure the RabbitMQ pod is running and ready before testing connectivity.
- If you encounter issues, check the pod logs and probe configurations.
- You can use the debug pod for other network or service connectivity tests as needed.

## Queue Service API Test

### Overview

The queue service API test is a comprehensive test suite that validates the functionality of the queue service API endpoints. It tests:

- Health check endpoint
- Queue operations (publish and status)
- Metrics endpoint

### Test Configuration

The test is configured using two main files:

1. `queue-service-api-test.yaml`: Defines the Kubernetes job that runs the test
2. `queue-service-api-test-configmap.yaml`: Contains the test script and configuration

### Running the Test

1. **Apply the ConfigMap:**

   ```bash
   kubectl apply -f k8s/debug/queue-service-api-test-configmap.yaml
   ```

2. **Run the Test Job:**

   ```bash
   kubectl apply -f k8s/debug/queue-service-api-test.yaml
   ```

3. **Check Test Results:**
   ```bash
   kubectl logs job/queue-service-api-test
   ```

### Test Coverage

The test suite validates:

- Service availability and health check
- Message publishing and status retrieval
- Prometheus metrics exposure
- Message validation
- Error handling

### Required Metrics

The test verifies the presence of these Prometheus metrics:

- `queue_messages_total`
- `queue_processing_duration_seconds`
- `queue_size`
- `queue_errors_total`
