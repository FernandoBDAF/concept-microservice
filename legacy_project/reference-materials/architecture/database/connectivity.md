# Database Connectivity Configuration

## Overview

This document describes the configuration and setup for connecting services running inside Kubernetes to a PostgreSQL database running outside the cluster (on the host machine).

## Architecture

```
[Kubernetes Pod] → [Kubernetes Service] → [Host Network] → [PostgreSQL on Host]
```

## Configuration Details

### 1. Host PostgreSQL Setup

The PostgreSQL database runs on the host machine with the following configuration:

- Port: 5432 (default)
- Host: 0.0.0.0 (allows connections from any IP)
- Authentication: password-based
- Database: profile_db
- User: profile_user

### 2. Kubernetes Configuration

#### Environment Variables

Services connect to PostgreSQL using the following environment variables:

```yaml
DB_HOST: "host.docker.internal" # Special DNS name for host machine
DB_PORT: "5432"
DB_NAME: "profile_db"
DB_USER: "profile_user"
DB_PASSWORD: "profile_password" # Stored in Kubernetes Secret
```

#### Kubernetes Secret

Database credentials are stored in a Kubernetes Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: postgres-credentials
type: Opaque
data:
  DB_PASSWORD: <base64-encoded-password>
```

#### ConfigMap

Database connection configuration is stored in a ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
data:
  DB_HOST: "host.docker.internal"
  DB_PORT: "5432"
  DB_NAME: "profile_db"
  DB_USER: "profile_user"
```

### 3. Connection Pooling

The service uses connection pooling with the following configuration:

```yaml
DB_MAX_OPEN_CONNS: "25"
DB_MAX_IDLE_CONNS: "5"
DB_CONN_MAX_LIFETIME: "5m"
DB_CONN_MAX_IDLE_TIME: "1m"
DB_CONN_RETRY_ATTEMPTS: "10"
DB_CONN_RETRY_INTERVAL: "5s"
```

### 4. Network Policy

A NetworkPolicy is configured to allow egress traffic to PostgreSQL:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-postgres-egress
spec:
  podSelector:
    matchLabels:
      app: profile-storage
  egress:
    - to:
        - ipBlock:
            cidr: 192.168.1.0/24 # Host network CIDR
      ports:
        - protocol: TCP
          port: 5432
```

## Troubleshooting

### Common Issues

1. **Connection Refused**

   - Verify PostgreSQL is running on the host
   - Check PostgreSQL is listening on 0.0.0.0
   - Verify port 5432 is open
   - Check host.docker.internal DNS resolution

2. **Authentication Failed**

   - Verify credentials in Kubernetes Secret
   - Check PostgreSQL user permissions
   - Verify database exists

3. **Connection Timeout**
   - Check network policies
   - Verify host network connectivity
   - Check firewall rules

### Debugging Steps

1. Test connection from inside the cluster:

```bash
kubectl run postgres-test --rm -it --image=postgres:15 -- bash
psql -h host.docker.internal -U profile_user -d profile_db
```

2. Check pod environment variables:

```bash
kubectl exec -it <pod-name> -- env | grep DB_
```

3. Verify network connectivity:

```bash
kubectl exec -it <pod-name> -- nc -zv host.docker.internal 5432
```

## Security Considerations

1. **Network Security**

   - Use NetworkPolicy to restrict access
   - Consider using a VPN or private network
   - Implement TLS for database connections

2. **Credential Security**

   - Store credentials in Kubernetes Secrets
   - Rotate credentials regularly
   - Use minimal required permissions

3. **Connection Security**
   - Use connection pooling
   - Implement connection timeouts
   - Monitor connection usage

## Best Practices

1. **Configuration**

   - Use environment variables for configuration
   - Store sensitive data in Kubernetes Secrets
   - Use ConfigMaps for non-sensitive data

2. **Connection Management**

   - Implement connection pooling
   - Use retry mechanisms
   - Monitor connection health

3. **Monitoring**
   - Monitor connection pool status
   - Track connection errors
   - Set up alerts for connection issues

## References

- [Kubernetes Secrets Documentation](https://kubernetes.io/docs/concepts/configuration/secret/)
- [PostgreSQL Connection Settings](https://www.postgresql.org/docs/current/runtime-config-connection.html)
- [Kubernetes Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
