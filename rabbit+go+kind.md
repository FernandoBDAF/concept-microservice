# Deploying RabbitMQ and Go Services on Kubernetes (Kind)

## Introduction

In this setup, we will deploy a RabbitMQ message broker on a local Kubernetes cluster (using kind) and two Go microservices: a publisher and multiple consumers. The publisher will send messages to multiple RabbitMQ queues, and each queue has a distinct consumer service processing those messages. The goal is to create a robust, scalable design that works for local testing and can be extended for production (with features like persistence, clustering, and security in the future). We will cover:

- A Kubernetes manifest for RabbitMQ (using a StatefulSet for potential scaling and persistence).
- Go AMQP client usage and idiomatic patterns for publishing to and consuming from specific queues (one queue per consumer).
- Dockerfiles and Kubernetes Deployment manifests for the Go publisher and consumer services.
- Service discovery and communication within the cluster (using a RabbitMQ Service DNS name).
- Structuring RabbitMQ exchanges, queues, and bindings to support multiple distinct consumers.
- Tips for local development with kind (e.g. loading images and testing message flow).

## RabbitMQ Deployment on Kubernetes

For a basic single-node RabbitMQ, you can use a Deployment. However, since RabbitMQ is a stateful service (it stores messages and can form clusters), a StatefulSet is recommended for a more robust setup and future scalability. A StatefulSet gives each RabbitMQ pod a stable network identity and can attach persistent storage for message durability. Even if we start with one replica, using a StatefulSet makes it easier to scale to a cluster later. Below is an example Kubernetes manifest for RabbitMQ using a StatefulSet. It runs the official RabbitMQ 3.x management image (which includes the management UI on port 15672) and sets up a headless service for stable DNS, as well as environment variables for default user credentials and clustering (if scaling later). We'll configure one replica for now and enable the management plugin.

// What is a headless service?
// In which scenerios would it make sense to scale. How would that scale be done? What would be possible challanges?

```yaml
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq # Service for RabbitMQ (headless for StatefulSet DNS)
spec:
  clusterIP: None # Headless service (needed for stable hostnames in StatefulSet)
  ports:
    - name: amqp
      port: 5672
      targetPort: 5672
    - name: http
      port: 15672
      targetPort: 15672
  selector:
    app: rabbitmq

---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
spec:
  serviceName: rabbitmq # Must match the headless service name
  replicas: 1 # start with 1; can scale to >1 for clustering
  selector:
    matchLabels:
      app: rabbitmq
  template:
    metadata:
      labels:
        app: rabbitmq
    spec:
      terminationGracePeriodSeconds: 10
      serviceAccountName: rabbitmq # (optional RBAC if needed for discovery)
      containers:
        - name: rabbitmq
          image: rabbitmq:3.11-management # RabbitMQ image with management UI
          env:
            - name: RABBITMQ_DEFAULT_USER # Set a default user for convenience
              value: "user"
            - name: RABBITMQ_DEFAULT_PASS
              value: "password"
            - name: RABBITMQ_ERLANG_COOKIE
              value: "mycookie" # Cookie for clustering (same across pods)
            - name: RABBITMQ_USE_LONGNAME
              value: "true" # Use long hostnames for clustering
            - name: RABBITMQ_NODENAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name # Each pod will be named rabbitmq-0, etc.
          ports:
            - name: amqp
              containerPort: 5672
            - name: http
              containerPort: 15672
          volumeMounts:
            - name: data
              mountPath: /var/lib/rabbitmq # Mount path for message storage
      volumes:
        - name: data
          emptyDir: {} # Ephemeral storage for dev (replace with persistent volume claim for production)
```

### Explanation

We created a StatefulSet named rabbitmq with one replica and a matching Service. The container uses the rabbitmq:3.11-management image (which has the management plugin enabled for the UI). We set environment variables RABBITMQ_DEFAULT_USER and RABBITMQ_DEFAULT_PASS to define credentials – this avoids the default guest/guest which cannot be used remotely by default. We also set RABBITMQ_ERLANG_COOKIE and some cluster-related settings to allow adding more RabbitMQ nodes later (the cookie must be the same on all nodes for a cluster). The broker's AMQP port 5672 and management port 15672 are exposed on the container. For storage, we use an emptyDir (ephemeral) volume for simplicity in local testing; in a real setup, you could use a PersistentVolumeClaim for durability. If persistence is needed in kind, you could use a hostPath volume or kind's default storage class (which uses local disk). Using a StatefulSet prepares us for future scaling. If you decide to run a RabbitMQ cluster (say 3 replicas), this manifest can be scaled up. When scaling, RabbitMQ's Kubernetes peer-discovery plugin (enabled via rabbitmq_peer_discovery_k8s in a config map) can help the nodes find each other. The above manifest doesn't explicitly include a ConfigMap, but you could add one to configure cluster_formation.peer_discovery_backend = rabbit_peer_discovery_k8s and related settings as shown in RabbitMQ's documentation (optional for single node). With multiple replicas, ensure the Erlang cookie is set (we used mycookie) so that the nodes can form a cluster. We also defined a headless Service (clusterIP: None) for RabbitMQ named "rabbitmq". This gives each pod a DNS name (e.g. rabbitmq-0.rabbitmq.default.svc.cluster.local) which RabbitMQ uses for clustering. For clients, we will create a normal ClusterIP Service next for easy access.

## RabbitMQ Service (Access within Cluster)

In addition to the headless service (used by the RabbitMQ pods for peer discovery), we typically create a normal Service to allow other applications to connect to RabbitMQ via a stable endpoint. We can reuse the same Service name for simplicity. In the manifest above, we already defined spec.ports for amqp and http on the Service. This Service (named rabbitmq) will be discoverable by other pods in the same namespace via DNS. For example, the AMQP URL for clients will be amqp://user:password@rabbitmq:5672/ (the hostname rabbitmq resolves to the service cluster IP). Within the cluster, no external exposure is needed (the service is of type ClusterIP), which means RabbitMQ is only reachable internally by the publisher/consumer pods. If you want to access the RabbitMQ Management UI on port 15672 from your host machine (for monitoring messages, queues, etc.), you can run:

```bash
kubectl port-forward service/rabbitmq 15672:15672
```

Then open http://localhost:15672 in a browser and log in with the user/password (user/password as set above). This is very useful to inspect exchanges, queues, and message rates during development. (Alternatively, you could set the Service type to NodePort or LoadBalancer in kind, but port-forward is simplest for local use.) Note: In a production scenario, you would enforce security (unique user credentials per service, possibly TLS). For now, we skip TLS and use a basic user for simplicity, as security is optional at this testing stage.

## Go Client Libraries and Idiomatic Patterns

For connecting Go applications to RabbitMQ, the idiomatic choice is the AMQP 0-9-1 client library. The officially maintained Go library is github.com/rabbitmq/amqp091-go (a community fork of the older streadway/amqp). This library allows you to dial an AMQP URL and work with connections, channels, exchanges, and queues.

### Connection & Channel management

It is considered best practice to use one long-lived TCP connection per process and open multiple channels on that connection for concurrency. Each channel represents a lightweight virtual connection (multiplexed over the TCP connection) and can be used by a separate goroutine for publishing or consuming. You should not frequently open/close connections or channels as that adds overhead. Instead, open a connection at service startup and reuse it. Also, do not share a single channel across concurrent goroutines, as most client implementations (including amqp091-go) are not thread-safe on a channel. Create separate channels if you have multiple publisher goroutines or use a synchronization mechanism. In summary, one connection per service (per pod), and e.g. one channel for all publishing from that service, and one channel per consumer routine, etc. Additionally, it's recommended to use separate connections for publishers and consumers if they run in the same process, because a slow consumer could otherwise block a publisher on the same connection due to TCP backpressure. In our case, the publisher and consumer are separate services (so naturally they use separate connections), but if you ever have a service that does both, keep this in mind.

### Go AMQP usage

The library's usage pattern is:

1. Dial the RabbitMQ server using the AMQP URL (which includes credentials, host, and port).
2. Open a channel on the connection.
3. Declare the necessary exchange and queue on that channel (queues and exchanges in RabbitMQ are idempotent – declaring them in multiple services is okay as long as parameters match).
4. For a publisher: Publish messages to an exchange (with a routing key). For a consumer: Consume messages from a queue (which subscribes the channel to receive messages).
5. Use acknowledgments appropriately (to ensure messages aren't lost or requeued as needed).

We will use environment variables to pass the RabbitMQ connection URL to our Go apps. For example, we might have AMQP_URL=amqp://user:password@rabbitmq:5672/ in the pod environment. The Go code will read this and call amqp.Dial on it.

Below is an example Go code snippet for the publisher service. This publisher sends messages to two different queues, "queue1" and "queue2", each intended for a different consumer. We'll use a direct exchange to route messages (more on the exchange setup in the next section).

```go
import (
    "log"
    "os"
    amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
    // Read AMQP URL from environment
    amqpURL := os.Getenv("AMQP_URL")
    if amqpURL == "" {
        amqpURL = "amqp://user:password@rabbitmq:5672/"  // default for local
    }

    // 1. Establish connection
    conn, err := amqp.Dial(amqpURL)
    if err != nil {
        log.Fatalf("Failed to connect to RabbitMQ: %v", err)
    }
    defer conn.Close()

    // 2. Create a channel
    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Failed to open channel: %v", err)
    }
    defer ch.Close()

    // 3. Declare an exchange (direct type) for routing messages
    err = ch.ExchangeDeclare(
        "tasks-exchange",   // exchange name
        "direct",           // type
        true,               // durable (survives broker restarts)
        false,              // autoDelete
        false,              // internal
        false,              // noWait
        nil,                // arguments
    )
    if err != nil {
        log.Fatalf("Failed to declare exchange: %v", err)
    }

    // 4. Declare two queues and bind them to the exchange with different routing keys
    queues := []string{"queue1", "queue2"}
    for _, qName := range queues {
        _, err := ch.QueueDeclare(
            qName,
            true,  // durable queue (survive restarts)
            false, // not auto-deleted
            false, // not exclusive (can be consumed by multiple consumers)
            false, // noWait
            nil,   // arguments (e.g., x-dead-letter-exchange if needed)
        )
        if err != nil {
            log.Fatalf("Failed to declare queue %s: %v", qName, err)
        }
        // Bind queue to exchange with routing key same as queue name
        err = ch.QueueBind(
            qName,
            qName,             // routing key
            "tasks-exchange",  // exchange
            false,
            nil,
        )
        if err != nil {
            log.Fatalf("Failed to bind queue %s: %v", qName, err)
        }
    }

    // 5. Publish messages to each queue via the exchange
    for i := 1; i <= 5; i++ {
        body := []byte("Hello Queue1 - msg " + fmt.Sprint(i))
        err = ch.Publish(
            "tasks-exchange", // exchange
            "queue1",         // routing key for queue1
            false,  // mandatory
            false,  // immediate
            amqp.Publishing{
                ContentType: "text/plain",
                DeliveryMode: amqp.Persistent,  // make message persistent
                Body:        body,
            })
        if err != nil {
            log.Printf("Failed to publish to queue1: %v", err)
        }

        body2 := []byte("Hello Queue2 - msg " + fmt.Sprint(i))
        _ = ch.Publish(
            "tasks-exchange",
            "queue2",
            false,
            false,
            amqp.Publishing{
                ContentType: "text/plain",
                DeliveryMode: amqp.Persistent,
                Body:        body2,
            })
    }

    log.Println("Published 5 messages to each queue")
}
```

// What if we want to scale the publisher of one particular queue?

In this publisher code, we:

1. Connect to RabbitMQ using the URL from AMQP_URL.
2. Declare a direct exchange named "tasks-exchange". The direct exchange routes messages to queues based on a routing key that exactly matches the queue's binding key. We choose direct because we want to explicitly route messages to specific queues (as opposed to fanout or topic).
3. Declare two durable queues, "queue1" and "queue2", and bind each to the exchange with the same name as the routing key (for simplicity, routing key "queue1" goes to queue1, etc.). Declaring the exchange/queues in the publisher ensures they exist; the consumers will similarly declare to be safe. Marking queues durable and messages persistent means that if RabbitMQ restarts, it will keep the queues and stored messages (useful for reliability).
4. Publish a few test messages to each queue via the exchange. Each message is published with the appropriate routing key, so it goes only to the intended queue. We set DeliveryMode: amqp.Persistent so that messages will be written to disk (in combination with durable queues) for persistence. In a real scenario, you might use Publisher Confirms to ensure the broker has received and persisted the messages, but for simplicity we omit confirm handling.

Next, here is a sample Go code for a consumer service. This consumer will connect to RabbitMQ and listen on one specific queue (let's say "queue1"). In practice, you'd have one such service (possibly with its own Deployment) for each queue (queue1, queue2, etc.), each running identical logic but handling different data. The code can be made flexible (e.g. read the queue name from an environment variable) so that the same container image can be used for multiple consumers, or you can hardcode the queue name if each consumer is built separately. Below example uses a queue name from env for reusability:

```go
import (
    "log"
    "os"
    amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
    amqpURL := os.Getenv("AMQP_URL")
    if amqpURL == "" {
        amqpURL = "amqp://user:password@rabbitmq:5672/"
    }
    queueName := os.Getenv("QUEUE_NAME")  // specify which queue this consumer handles
    if queueName == "" {
        log.Fatal("QUEUE_NAME not set")
    }

    // 1. Connect and open channel
    conn, err := amqp.Dial(amqpURL)
    if err != nil {
        log.Fatalf("Connection error: %v", err)
    }
    defer conn.Close()
    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Channel error: %v", err)
    }
    defer ch.Close()

    // 2. Declare the queue (should match the publisher's declaration)
    _, err = ch.QueueDeclare(
        queueName,
        true,    // durable (must match publisher's declaration)
        false,   // autoDelete
        false,   // exclusive
        false,   // noWait
        nil,
    )
    if err != nil {
        log.Fatalf("Queue Declare error: %v", err)
    }

    // (Optional) set QoS to process one message at a time (fair dispatch)
    ch.Qos(1, 0, false)

    // 3. Start consuming messages
    msgs, err := ch.Consume(
        queueName,
        "",     // consumer tag (empty -> auto-generated)
        false,  // auto-acknowledge? (false means we'll ack manually)
        false,  // exclusive
        false,  // no-local (not applicable in most cases)
        false,  // noWait
        nil,
    )
    if err != nil {
        log.Fatalf("Queue Consume error: %v", err)
    }

    log.Printf("Consumer up, waiting for messages on queue: %s", queueName)
    // 4. Process messages
    for msg := range msgs {
        log.Printf("Received on %s: %s", queueName, string(msg.Body))
        // TODO: process the message (business logic)
        msg.Ack(false)  // acknowledge message after processing
    }
}
```

Key points in the consumer code:

1. It uses the same AMQP_URL to connect. It also expects an environment variable QUEUE_NAME to know which queue to consume. For example, one deployment could set QUEUE_NAME=queue1 and another queue2, both using the same container image.
2. It declares the queue (this will succeed if it already exists with the same settings). Declaring on the consumer side ensures the queue is present even if the publisher hasn't run yet. Both publisher and consumer declare the same queue, which is fine as long as they agree on parameters.
3. We call ch.Qos(1,0,false) to set a prefetch count of 1. This tells RabbitMQ not to give this consumer more than one unacknowledged message at a time. This is a fair dispatch setting: the consumer won't be overwhelmed and messages won't all go to one fast consumer. You can tune prefetch for performance – a higher value can improve throughput, but 1 ensures each consumer gets one message at a time until it ack's it.
4. We start consuming with autoAck=false. This means the consumer must manually acknowledge messages (msg.Ack(false)) after processing. By not auto-acking, we ensure that if our consumer crashes or fails to process a message, the message will not be lost – RabbitMQ will requeue it for another consumer or a restarted consumer. This is important for reliability (at-least-once delivery). If we had used autoAck=true (as in quick demos), the broker would consider messages delivered as soon as they're sent out, which can lose messages if a consumer dies mid-processing.
5. The loop reads from the channel msgs. This will continuously receive messages as they arrive. We simply log the message body and then ack the message. In a real service, you'd replace the log with actual processing logic. The loop will run until the program is stopped or the channel is closed.

By running one instance of this consumer code for queue1 and another for queue2, we have isolated consumers – each only processes the messages intended for its queue. This one queue per consumer model also guarantees message ordering per queue (no parallel consumers on the same queue to interleave messages). Using a single consumer per queue ensures FIFO processing for that queue (messages in a queue are delivered sequentially to its single consumer). If we needed to scale up processing of a particular queue, we could run multiple replicas of the same consumer deployment (all with the same QUEUE_NAME). RabbitMQ would then distribute messages from that queue among the consumer instances (competing consumers) in a round-robin fashion. That can increase throughput, but note that with multiple consumers on one queue, ordering is not guaranteed across different consumers (each consumer will see a subset of messages). For our design, we stick to one consumer per queue to keep processing logic independent and ordered.

## RabbitMQ Exchanges, Queues, and Routing Strategy

To support multiple distinct consumers with one publisher, the messaging topology should be designed for explicit routing. In RabbitMQ, producers send messages to an exchange rather than directly to a queue. The exchange then routes each message to one or more queues based on routing rules (bindings and routing keys). For our scenario (one message goes to one specific consumer's queue), a direct exchange is most suitable. A direct exchange delivers a message to the queue whose binding key exactly matches the message's routing key. In our example, we declared a direct exchange called "tasks-exchange". We bind each queue to this exchange using a unique routing key per queue (we conveniently used the queue's name as the routing key in the code above). The publisher, when sending a message intended for Consumer A, will publish to "tasks-exchange" with routing key "queue1", and the exchange will route that message to Queue1 (and only Queue1, since that queue's binding key matches). Similarly, messages for Consumer B use routing key "queue2", routed to Queue2. This setup ensures that each consumer only receives the messages meant for it. The producer doesn't have to know the queue names explicitly if you abstract the routing keys as message types or topics, but in our simple case they coincide. This approach is scalable: if in the future you add a new consumer and queue, you can bind it to the exchange with a new routing key without affecting existing consumers. The publisher just needs to use the new routing key for relevant messages. It also allows multiple queues to be bound with the same routing key if you ever needed fan-out behavior (though usually a fanout exchange or a topic exchange with wildcards is used for broadcast scenarios). For one-to-one routing, direct exchanges keep things clear and efficient. (Note: You could publish directly to a queue without specifying an exchange – by using the empty string "" which means the default direct exchange – this would route by queue name implicitly. However, defining your own exchange is a cleaner design and more flexible for future changes.)

### Durability and Persistence

We have set queues as durable and used persistent messages in our examples. This means that RabbitMQ will store the messages on disk, so if the broker restarts or crashes, it can recover the queues and messages. This is important for not losing data. However, note that durable/persistent doesn't guarantee no message loss in a cluster unless you use mirrored or quorum queues. If you later scale RabbitMQ to a cluster of nodes, consider using quorum queues (a RabbitMQ 3.8+ feature for replicated, consistent storage) for high reliability, or classic mirrored queues (which require configuring a policy). For local testing, a single durable queue is fine.

### Bindings and Topics

If you foresee having categories of consumers or wildcard routing, you might use a topic exchange instead of direct. A topic exchange allows routing keys like "order.created" or "user.\*" to match multiple queues by pattern. This can be useful if some consumers want to subscribe to a broader set of messages. In our one-queue-per-consumer scenario, a direct exchange is sufficient, but keep in mind the flexibility of RabbitMQ exchanges if requirements grow. Finally, if a consumer is meant to receive all messages of a certain type in parallel with another consumer (i.e., publish/subscribe pattern where one message should be processed by multiple different services), you could bind multiple queues to the same routing key or use a fanout exchange (which broadcasts to all bound queues). That is a different pattern (one message to many consumers) and would require multiple queues for the same message. Our design currently is one message to one consumer (just via dedicated queues).

## Dockerfile for Go Publisher and Consumer Services

Both the publisher and consumer are Go programs, which we will containerize for Kubernetes. We will use a multi-stage Docker build to create small, efficient images. In the build stage, we compile the Go binary, then in the final stage we use a minimal base (such as Alpine or scratch). Here is a generic Dockerfile that can be adapted for both publisher and consumer (by copying the respective source code).

```dockerfile
# Stage 1: Build the Go binary
FROM golang:1.20-alpine AS builder
WORKDIR /app

# Configure Go modules and dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (adjust path to your main.go)
COPY ./src/*.go ./   # assume main package files
# Alternatively, copy a specific directory for publisher or consumer

# Build the binary
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -o app-binary .

# Stage 2: Create a lightweight container
FROM scratch
# If using scratch, no base OS – just the binary. For Alpine, use e.g. FROM alpine:3.18
# Copy the compiled binary from the builder
COPY --from=builder /app/app-binary /app/app-binary
# Set the entrypoint
ENTRYPOINT ["/app/app-binary"]
```

In the above Dockerfile, we used golang:1.20-alpine as the builder image (any recent Go version works). We copied the Go module files and source, then built the binary with CGO_ENABLED=0 (so it's a static binary, suitable for scratch). The final image is FROM scratch which is minimal – it contains only the binary and no OS libraries. The image will therefore be very small (just a few MB). We set the entrypoint to run the binary. You would build this image for the publisher (ensuring the source copied is the publisher's code) and similarly for the consumer. For clarity, you might have two Dockerfiles or one Dockerfile that builds both binaries (one at a time). For example, you could have Dockerfile.publisher and Dockerfile.consumer or simply use build args to specify the target. In our demonstration, having separate ones is fine. The multi-stage build ensures we don't include the Go compiler or source in the final runtime image. After building the Docker images (e.g., publisher:latest and consumer:latest tags locally), you need to make them available to the kind cluster. Since kind runs its own Docker daemon inside the node container, the images aren't automatically present. The simplest way is to use the command:

```bash
kind load docker-image publisher:latest --name <your-cluster-name>
kind load docker-image consumer:latest --name <your-cluster-name>
```

This will transfer the local images into the kind cluster nodes so Kubernetes can pull them. (Alternatively, you could push the images to a registry and have Kubernetes pull them, but for local dev, loading them directly is easiest.) Once loaded, you can reference these images in your Pod specs without needing an external registry.

## Kubernetes Manifests for Publisher and Consumer Deployments

With the RabbitMQ service running, we deploy our Go services into the cluster. We will create a Deployment for the publisher and one Deployment per consumer service/queue. Each Deployment will have a Pod template that includes the container image and environment variables for configuration (like the RabbitMQ URL and queue name).

### Publisher Deployment example

This deployment runs a single replica of the publisher (you might scale this up if the publisher needs to handle a lot of external events, but typically one is enough for a scheduler/dispatcher type service). The publisher might not need to be exposed outside the cluster if it generates messages internally (e.g., timed events or triggered by internal logic). If the publisher needs to receive external requests (like an API server), you can also create a Service for it (ClusterIP or NodePort as needed). In our case, let's assume the publisher is self-driven or not externally called, so no external service is required.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: publisher
spec:
  replicas: 1
  selector:
    matchLabels:
      app: publisher
  template:
    metadata:
      labels:
        app: publisher
    spec:
      containers:
        - name: publisher
          image: publisher:latest # image loaded into kind
          env:
            - name: AMQP_URL
              value: "amqp://user:password@rabbitmq:5672/" # RabbitMQ connection URL
          # (If this publisher needed to know which queues to use, you could also pass those via env)
```

This manifest schedules the publisher pod and sets the AMQP_URL environment variable. We use the RabbitMQ service's hostname rabbitmq (since it's in the same namespace) and the credentials we set up (user:password). The publisher container on startup will use this to connect.

### Consumer Deployment example

For each consumer service, we create a similar deployment. Suppose we have two consumer services corresponding to "queue1" and "queue2". We can either use the same container image but supply different QUEUE_NAME env, or build two separate images if the consumers have different logic compiled in. If the consumer logic is generic or toggled by queue, using one image is convenient. We'll assume one image that can handle different queues by env. We will create two deployments consumer-q1 and consumer-q2 for illustration:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: consumer-q1
spec:
  replicas: 1 # start with one consumer instance
  selector:
    matchLabels:
      app: consumer-q1
  template:
    metadata:
      labels:
        app: consumer-q1
    spec:
      containers:
        - name: consumer
          image: consumer:latest
          env:
            - name: AMQP_URL
              value: "amqp://user:password@rabbitmq:5672/"
            - name: QUEUE_NAME
              value: "queue1"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: consumer-q2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: consumer-q2
  template:
    metadata:
      labels:
        app: consumer-q2
    spec:
      containers:
        - name: consumer
          image: consumer:latest
          env:
            - name: AMQP_URL
              value: "amqp://user:password@rabbitmq:5672/"
            - name: QUEUE_NAME
              value: "queue2"
```

Each consumer deployment is nearly identical except for the QUEUE_NAME value. They each connect to RabbitMQ at the same service (rabbitmq:5672) and then listen on their respective queue. We keep one replica of each (one pod per queue) to preserve the single-consumer FIFO processing model. If later you find that one queue has too many messages building up, you can scale its deployment to 2 or 3 replicas. RabbitMQ will then load-balance messages from that queue among the pods (each pod still calls Consume on the same queue). This is how you achieve horizontal scaling of consumers without introducing new queues. Just remember, multiple consumers on one queue = parallel processing without global order guarantee. We did not create separate Services for the consumer pods because they don't need to be reached from outside – they initiate the connection to RabbitMQ on their own. They will simply log or process data. If a consumer service needed to expose an API or be called by other services, you would add a Service for it, but that's beyond our current scope.

### Environment config

Hardcoding the AMQP URL as we did in env value is fine for local test. For better practice, you might store the credentials in a Secret and reference it, or at least not embed plaintext in the manifest. However, since security was not the focus here, we used the straightforward approach. The RabbitMQ host is rabbitmq (the service DNS name). If the publisher/consumer were in a different namespace than RabbitMQ, you'd have to use the full DNS (rabbitmq.<namespace>.svc.cluster.local). But here we assume all are deployed to the same namespace (e.g., "default" or "test"). Kubernetes will automatically inject DNS search domains so that just rabbitmq will resolve to the service IP in the same namespace.

## In-Cluster Communication and Verification

With the manifests above, the networking is handled by Kubernetes – the publisher and consumers find RabbitMQ by the service name. No manual IP management is needed. Kubernetes DNS ensures that rabbitmq points to the RabbitMQ pod(s). When the publisher/consumer dial amqp://user:password@rabbitmq:5672/, that connection is routed inside the cluster to the RabbitMQ pod on port 5672. To verify that everything works in your kind cluster, you can follow these steps:

1. Apply the manifests: Use kubectl apply -f rabbitmq-manifest.yaml (for the RabbitMQ StatefulSet and service) and ensure the RabbitMQ pod is running (kubectl get pods). Then apply the publisher and consumer deployment manifests. After a short time, all pods should be in Running state.
2. Check RabbitMQ readiness: The StatefulSet manifest included a readiness probe (if you added one, e.g., using rabbitmq-diagnostics ping). Make sure the RabbitMQ pod is ready before expecting the consumers to connect. The consumers/publisher will likely retry if RabbitMQ isn't up yet (the amqp library by default will error if can't connect; you might want to implement a retry loop with backoff in real apps).
3. View Logs: Use kubectl logs to check what's happening. For example, kubectl logs deploy/publisher should show if it successfully published messages. The consumer logs (kubectl logs deploy/consumer-q1) should show messages being received. For instance, you might see logs like "Published 5 messages to each queue" from the publisher, and "Received on queue1: Hello Queue1 – msg 1" from the consumer. This confirms end-to-end flow.
4. Management UI: As mentioned, port-forward RabbitMQ's 15672 if you want a visual check. On the UI, you'll see the defined exchange ("tasks-exchange"), the queues, bindings, and you can observe the message rates. You can even publish test messages via the UI (in the Exchange section, there's a form to publish a message to an exchange with a routing key, or in the Queue section, you can publish directly to a queue). This can be helpful for manual testing.
5. Testing message flow: Another way to test is to exec into the publisher pod or use a simple job to send messages. But since our publisher code already sends some messages on start (in the example above, 5 messages to each queue), the test is mostly automatic. If you want the publisher to run continuously or periodically, you could modify it to, say, publish messages in a loop or on a schedule. For a quick test, the static loop is fine. The consumer will keep running, waiting for messages. You can also test failure scenarios: try killing a consumer pod (kubectl delete pod <consumer-pod>), and watch RabbitMQ queue length increase (via UI or kubectl logs publisher showing messages still publishing). When the consumer pod comes back, it should receive the pending messages (since we used durable queue and manual ack, messages stay in the queue until consumed). The dev setup thus simulates how things would work in production with robustness to consumer restarts.

### Local development iteration tips

When you update your Go code, you'll need to rebuild the image and load it into kind again. It's useful to give it a new tag (or use :latest but force redeploy). After loading the image, you may need to delete the existing pod or set the imagePullPolicy to Never (Kubernetes by default might cache the image since there's no registry pull). In a dev workflow, one can also use ko or Skaffold for rapid development, but with kind load and kubectl rollout restart deployment/<name> you can manually refresh pods with the new images. Finally, since this is a single-node kind cluster, there's no real difference in scheduling. All pods run on the one node (which is fine). If you configured kind with multiple nodes, the headless service and statefulset will ensure that if RabbitMQ runs on node1, and a consumer on node2, they still communicate via the service without issues.

## Conclusion

By deploying RabbitMQ with a proper Kubernetes manifest and connecting Go-based publisher/consumer services, we achieve a functional event-driven system in a local kind cluster. The RabbitMQ StatefulSet ensures our broker is set up in a way that we can extend to a cluster or add persistence easily. Each Go service uses the amqp091 Go client with recommended patterns: one connection per service and durable messaging setup. We structured the messaging with an exchange and distinct queues per consumer, which provides isolation and the ability to scale or add new consumers without disrupting others. For local development, kind proved to be a convenient tool – we can build Docker images locally, load them into the cluster, and test the whole pipeline on our machine. The management UI and kubectl logs help in verifying that the publisher is sending messages to the right queues and each consumer is receiving only its intended messages. This design lays the groundwork for future enhancements: for example, adding TLS/auth on RabbitMQ (you could inject certs and use amqps:// URLs), deploying the RabbitMQ Cluster Operator for production-grade management, or configuring DLQs (Dead Letter Queues) and retry mechanisms for failed messages. By following these practices, the system will be robust even as you scale up: you can increase the number of queues/consumers (vertical scalability by adding more types of messages), or scale out each consumer horizontally for throughput, and even run RabbitMQ in a cluster for high availability. This approach provides a clean separation of concerns and is well-suited for a microservices architecture where each service handles a specific subset of messages.

### Sources

The solution incorporates RabbitMQ best practices (e.g. connection/channel use) and concepts of exchanges and routing keys. The one-queue-per-consumer model is in line with preserving FIFO processing per consumer. The provided code and manifest examples were informed by official RabbitMQ tutorials and community examples, adapted for our specific scenario. This ensures that our design is grounded in proven approaches and will serve as a solid foundation for more advanced messaging workflows.
