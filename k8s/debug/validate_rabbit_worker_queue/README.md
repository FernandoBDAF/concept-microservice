# Validate RabbitMQ Worker Queue Test

This folder contains a test environment for validating the RabbitMQ worker queue flow. The test consists of the following components:

- **RabbitMQ**: A StatefulSet for RabbitMQ with clustering support, Prometheus metrics, and a headless service.
- **Worker**: A Deployment for the worker service, configured to connect to the RabbitMQ test instance.
- **Queue Service**: A Deployment for the queue service, also configured to connect to the RabbitMQ test instance.
- **Test Job**: A Job to run the test, configured to connect to the queue-service-test.

## Prerequisites

- Kubernetes cluster (e.g., Minikube, Kind, or a cloud provider)
- `kubectl` installed and configured
- Docker images for `worker-service`, `queue-service`, and `test-job` built and available in your cluster

## Deployment

1. **Build and push the Docker images**:

   ```bash
   # Build the worker-service image
   docker build -t worker-service:latest ./path/to/worker-service

   # Build the queue-service image
   docker build -t queue-service:latest ./path/to/queue-service

   # Build the test-job image
   docker build -t test-job:latest -f k8s/debug/validate_rabbit_worker_queue/Dockerfile.test-job ./path/to/test-job

   # Push the images to your registry (if needed)
   docker push worker-service:latest
   docker push queue-service:latest
   docker push test-job:latest
   ```

2. **Deploy the test environment**:

   ```bash
   kubectl apply -k k8s/debug/validate_rabbit_worker_queue
   ```

3. **Verify the deployment**:

   ```bash
   kubectl get pods
   ```

   Ensure all pods are in the `Running` state.

## Running the Test

The test job will automatically start once the environment is deployed. You can check the logs of the test job using:

```bash
kubectl logs -l app=test-job
```

## Cleanup

To clean up the test environment, run:

```bash
kubectl delete -k k8s/debug/validate_rabbit_worker_queue
```

## Troubleshooting

- **Pods not starting**: Check the logs of the pods using `kubectl logs <pod-name>`.
- **RabbitMQ not connecting**: Ensure the RabbitMQ service is running and the worker/queue-service pods can reach it.
- **Test job failing**: Check the logs of the test job for errors.

For further assistance, refer to the Kubernetes documentation or contact the team.
