# Network Policies

## Overview

Network policies in our microservices architecture define how pods can communicate with each other and with external resources. They are crucial for implementing the principle of least privilege and ensuring secure service-to-service communication.

## Profile Storage Service Network Policy

### Configuration

The Profile Storage Service uses a NetworkPolicy to control both ingress and egress traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: profile-storage-network-policy
spec:
  podSelector:
    matchLabels:
      app: profile-storage
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: microservice
      ports:
        - protocol: TCP
          port: 50051 # gRPC port
        - protocol: TCP
          port: 8080 # REST API port
  egress:
    - to:
        - ipBlock:
            cidr: 192.168.86.115/32 # External PostgreSQL database
      ports:
        - protocol: TCP
          port: 5432
```

### Traffic Rules

1. **Ingress Traffic**

   - Allows incoming connections from any pod within the `microservice` namespace
   - Permits traffic on ports:
     - 8080: REST API
     - 50051: gRPC

2. **Egress Traffic**
   - Allows outbound connections to the external PostgreSQL database
   - Permits traffic on port 5432

### Security Considerations

- The policy follows the principle of least privilege
- Only necessary ports are exposed
- External database access is restricted to a specific IP
- Internal service communication is restricted to the microservice namespace

## Best Practices

1. **Namespace Isolation**

   - Use namespace selectors to restrict cross-namespace communication
   - Label namespaces appropriately for policy targeting

2. **Port Management**

   - Only expose required ports
   - Use named ports for better maintainability
   - Document port usage and purpose

3. **External Access**

   - Use IP blocks for external resource access
   - Document external dependencies
   - Consider using service mesh for complex external communication

4. **Policy Maintenance**
   - Review policies regularly
   - Update policies when service requirements change
   - Document policy changes and their rationale

## Troubleshooting

### Common Issues

1. **Connection Refused**

   - Check if the network policy allows the required ports
   - Verify namespace selectors are correctly configured
   - Ensure pod labels match the policy selectors

2. **External Access Issues**
   - Verify IP block CIDR ranges
   - Check if external resources are accessible
   - Ensure DNS resolution is working

### Debugging Commands

```bash
# Check network policy status
kubectl get networkpolicy -n microservice

# Describe specific policy
kubectl describe networkpolicy profile-storage-network-policy -n microservice

# Test connectivity from a pod
kubectl exec -it <pod-name> -n microservice -- curl <service-name>:<port>
```

## Future Improvements

1. **Service Mesh Integration**

   - Consider implementing Istio or Linkerd
   - Enhanced traffic management
   - Improved security features

2. **Policy Automation**

   - Generate policies from service definitions
   - Automated policy testing
   - Policy validation in CI/CD

3. **Monitoring and Logging**
   - Network policy violation logging
   - Policy effectiveness metrics
   - Automated policy optimization
