# Step-by-Step Deployment Guide Template

## 📖 **Purpose of This Document**

This document provides **guidelines and templates** for creating comprehensive step-by-step deployment guides for microservices. Use this as a reference when creating `STEP_BY_STEP_DEPLOYMENT_GUIDE.md` files for other services.

## 🎯 **Philosophy & Goals**

### **Primary Objectives**

1. **Educational**: Help developers understand each Kubernetes resource and its impact
2. **Troubleshooting**: Enable step-by-step problem isolation and resolution
3. **Environment-Aware**: Provide appropriate guidance for different cluster types
4. **Comprehensive**: Cover all deployment scenarios and common issues

### **Target Audience**

- **New developers** learning Kubernetes and microservices
- **Experienced developers** troubleshooting deployment issues
- **Operations teams** understanding service dependencies
- **Anyone** wanting to understand the deployment process deeply

### **Core Principles**

- **Learning-First**: Prioritize understanding over speed
- **Environment-Appropriate**: Kind-optimized for development, production-aware
- **Problem-Solving**: Include extensive troubleshooting guidance
- **Practical**: Provide real commands and expected outputs

## 📁 **File Structure Template**

```markdown
# Step-by-Step Kubernetes Deployment Guide

## [Service Name] [Architecture Description]

Brief description of what this guide covers and the service's role.

## 🚀 Two Ways to Follow This Guide

### Option 1: Automated Manual Deployment (Recommended)

### Option 2: Manual Commands (Educational)

## 📋 Prerequisites

## 🚀 Deployment Sequence

### Step 1: [Foundation Resources]

### Step 2: [Configuration Resources]

### Step 3: [Network & Security Resources]

### Step 4: [Application Resources]

### Step 5: [Monitoring Resources]

## 🔍 Comprehensive Cluster State Commands

## 🎯 What to Look For at Each Step

## 🚨 Common Issues & Troubleshooting

## 🧪 Quick Test Suite

## 📝 Notes
```

## 🛠️ **Section-by-Section Guide**

### **1. Header & Introduction**

```markdown
# Step-by-Step Kubernetes Deployment Guide

## [Service Name] [Key Architecture Feature]

This guide walks you through deploying each Kubernetes manifest individually,
helping you understand the impact of each component on your cluster.
```

**Guidelines**:

- **Service Name**: Use the official service name
- **Architecture Feature**: Highlight key architectural aspects (e.g., "Multi-Worker Architecture", "Event-Driven Processing")
- **Brief Description**: 1-2 sentences explaining the service's role in the ecosystem

### **2. Two-Option Approach**

````markdown
## 🚀 Two Ways to Follow This Guide

### Option 1: Automated Manual Deployment (Recommended)

Use the automated manual deployment script that follows this guide:

```bash
cd deployments/scripts

# Interactive step-by-step deployment
./manual-deploy.sh --step-by-step

# With detailed manifest analysis
./manual-deploy.sh --analyze

# Cleanup when done
./manual-cleanup.sh --step-by-step
```
````

**⚠️ Important Note**: The manual deployment script **automatically detects** your cluster type:

- **Kind clusters**: Uses Kind-optimized settings (appropriate for local development)
- **Production clusters**: Uses full production settings

### Option 2: Manual Commands (Educational)

Follow the detailed commands below to understand each step completely.

**⚠️ Note**: These manual commands use **Kind-optimized manifests**. For production deployment, use Option 1 or `kubectl apply -f deployments/kubernetes/`.

````

**Guidelines**:
- **Always include both options** - automated script and manual commands
- **Explain the difference** in cluster detection and optimization
- **Set expectations** about which approach is better for which use case

### **3. Prerequisites Section**

```markdown
## 📋 Prerequisites

Ensure you have a kind cluster running and context set:

```bash
# Check if kind cluster exists
kind get clusters

# If not, create one (or use existing)
kind create cluster --name microservices

# Set context
kubectl config use-context kind-microservices

# Verify cluster access
kubectl cluster-info
kubectl get nodes
````

````

**Guidelines**:
- **Check cluster existence** before starting
- **Provide creation commands** for consistency
- **Include verification steps** to ensure readiness
- **Use consistent cluster naming** across all services

### **4. Deployment Sequence Section**

```markdown
## 🚀 Deployment Sequence

**🎯 Target Environment**: This guide is optimized for **Kind (local development)** clusters.

For **production deployment**, use:
```bash
kubectl apply -f deployments/kubernetes/
kubectl apply -f deployments/monitoring/
````

The steps below walk through **Kind-optimized deployment** for educational purposes:

````

**Step Template**:
```markdown
### Step X: [Icon] [Action] ([Resource Type])

**What it does**: [Clear explanation of purpose and impact]

#### Deploy:

```bash
[Kind-appropriate deployment commands]
````

**⚠️ Why [Optimization]?** [Explanation of any special considerations]

#### Observation Commands:

```bash
# 1. [Primary verification command]
[command with brief explanation]

# 2. [Resource inspection command]
[command with brief explanation]

# 3. [Detailed analysis command]
[command with more detailed inspection]

# 4. [Cross-verification command]
[command to verify integration with other resources]

# 5. [Event checking command]
[command to check for any issues]
```

**Expected Impact**: ✅ [Clear statement of what should be created/configured]

````

**Guidelines for Steps**:
- **Logical Order**: Foundation → Configuration → Network → Application → Monitoring
- **Clear Icons**: Use consistent icons (🔐 for secrets, ⚙️ for config, 🚀 for apps, etc.)
- **Explain Purpose**: Each step should clearly explain "what it does" and "why it matters"
- **Kind-Optimized**: Use Kind-appropriate commands, not production ones
- **Rich Observation**: Provide 5+ commands for thorough understanding
- **Expected Impact**: Clear statement of success criteria

### **5. Resource-Specific Guidelines**

#### **Secrets (Step 1)**
```markdown
### Step 1: 🔐 Deploy Secrets (`secrets.yaml`)

**What it does**: Creates sensitive configuration data (passwords, keys, tokens)

#### Deploy:
```bash
kubectl apply -f deployments/kubernetes/secrets.yaml
````

#### Observation Commands:

```bash
# 1. Check if secrets were created
kubectl get secrets
kubectl get secrets -l app=[service-name]

# 2. Describe the secrets (shows metadata, not actual values)
kubectl describe secret [service-name]-secrets
kubectl describe secret [service-name]-secrets-local

# 3. Check secret data keys (still encoded)
kubectl get secret [service-name]-secrets -o yaml

# 4. Decode a secret value (example - DON'T do this in production!)
kubectl get secret [service-name]-secrets -o jsonpath='{.data.DB_USER}' | base64 --decode && echo

# 5. Watch for any events
kubectl get events --sort-by=.metadata.creationTimestamp --field-selector involvedObject.kind=Secret
```

**Expected Impact**: ✅ Two secrets created (`[service-name]-secrets` and `[service-name]-secrets-local`)

````

#### **ConfigMaps (Step 2)**
- Focus on configuration data structure
- Show how to inspect configuration values
- Explain service-specific configuration options

#### **Service & RBAC (Step 3)**
- Cover ServiceAccount, ClusterRole, Service creation
- Include HPA, PDB, NetworkPolicy if applicable
- Explain security and networking implications

#### **Application Deployment (Step 4)**
```markdown
### Step 4: 🚀 Deploy Application (Kind-Optimized)

**What it does**: Creates the [Service Name] application pods optimized for Kind development

#### Deploy:
```bash
# Option A: Use Kustomize (Recommended for Kind)
kubectl apply -k deployments/kind/

# Option B: Apply only the deployment from kustomized output
kubectl kustomize deployments/kind/ | grep -A 200 "kind: Deployment" | kubectl apply -f -
````

**⚠️ Why Kind-Optimized?** This uses:

- **[X] replica(s)** - suitable for single-node Kind
- **Reduced resources** - appropriate for local development
- **Local secrets** - uses `[service-name]-secrets-local`
- **Debug logging** - easier troubleshooting
- **[Service-specific optimizations]**

#### Critical Observation Commands:

```bash
# 1. Watch the deployment rollout in real-time
kubectl rollout status deployment/[service-name] --timeout=300s

# 2. Check deployment status
kubectl get deployments [service-name]
kubectl describe deployment [service-name]

# 3. Check pods (the most important!)
kubectl get pods -l app=[service-name]
kubectl get pods -l app=[service-name] -o wide

# 4. Describe a specific pod to see what's wrong
kubectl describe pods -l app=[service-name]

# 5. Check pod logs (this is where you'll see if the app is working)
kubectl logs -l app=[service-name] --tail=50
kubectl logs -l app=[service-name] -f  # Follow logs in real-time

# 6. Check if service endpoints are now populated
kubectl get endpoints [service-name]
kubectl describe endpoints [service-name]

# 7. [Service-specific health checks]
[commands specific to the service's functionality]
```

**Expected Impact**: ✅ [X] pod(s) created, running the [Service Name] application

````

#### **Monitoring (Step 5)**
- Prioritize Kind-appropriate monitoring
- Explain Prometheus Operator dependency
- Provide fallback options

### **6. Comprehensive State Commands**

```markdown
## 🔍 Comprehensive Cluster State Commands

After deploying all manifests, use these commands to see the complete picture:

```bash
# 1. Overview of all resources
kubectl get all -l app=[service-name]

# 2. Complete resource inventory
kubectl get secrets,configmaps,serviceaccounts,services,deployments,pods,hpa,pdb,networkpolicies -l app=[service-name]

# 3. Pod health and readiness status
kubectl get pods -l app=[service-name] -o wide

# 4. Service connectivity test
kubectl get services [service-name]
kubectl get endpoints [service-name]

# 5. Application logs (most important for debugging)
kubectl logs -l app=[service-name] --tail=100 --all-containers=true

# 6. [Service-specific verification commands]
[commands to test the service's core functionality]

# 7. Complete cluster events (for troubleshooting)
kubectl get events --sort-by=.metadata.creationTimestamp --all-namespaces | tail -50
````

````

### **7. What to Look For Section**

```markdown
## 🎯 What to Look For at Each Step

### Step 1 (Secrets):
- ✅ [Expected number] secrets created successfully
- ✅ No error events
- ✅ Secrets contain expected keys ([list key names])

### Step 2 (ConfigMaps):
- ✅ [Expected number] ConfigMaps with [service] configuration
- ✅ Check that [key config values] are properly set
- ✅ [Service-specific configuration validations]

### Step 3 (Service/RBAC):
- ✅ Service created with ClusterIP
- ✅ ServiceAccount has proper RBAC permissions
- ✅ [Additional networking/security validations]

### Step 4 (Deployment):
- ⚠️ **CRITICAL**: Pods transition: Pending → ContainerCreating → Running
- ⚠️ **CRITICAL**: [X] replica(s) become Ready (1/1)
- ✅ Service endpoints populated with pod IPs
- ✅ Application logs show successful startup
- ✅ [Service-specific health checks] respond
- ❌ **TROUBLESHOOT**: If pods stuck in Pending/CrashLoopBackOff

### Step 5 (Monitoring):
- ✅ [Monitoring resources] created
- ✅ Metrics endpoint accessible
- ✅ [Service-specific monitoring validations]
````

### **8. Troubleshooting Section**

**Structure**:

```markdown
## 🚨 Common Issues & Troubleshooting

### [Issue Category] (e.g., Architecture Mismatch, Image Pull Issues)

**Issue**: [Clear description of the problem and symptoms]
```

[Example error message or symptom]

````

**Root Cause**: [Explanation of why this happens]

**Solution**:
```bash
[Step-by-step commands to fix the issue]
````

**Prevention**: [How to avoid this issue in the future]

````

**Common Categories**:
- Architecture Mismatch (ARM64 vs AMD64)
- Image Pull Issues
- Missing Dependencies and Secrets
- Application Configuration Issues
- DNS Resolution Issues
- Resource Constraints
- Service-Specific Issues

### **9. Quick Test Suite**

```markdown
## 🧪 Quick Test Suite

After everything is deployed and running:

```bash
# 1. Basic connectivity test
kubectl port-forward service/[service-name] [port]:[port] &
sleep 2

# 2. Health check
curl -f http://localhost:[port]/health && echo "✅ Health OK" || echo "❌ Health Failed"

# 3. [Service-specific functional tests]
[Commands to test the service's core functionality]

# 4. Check metrics
curl -s http://localhost:[port]/metrics | grep [service-name]_ | wc -l | xargs echo "[Service] metrics count:"

# Cleanup port-forward
pkill -f "kubectl port-forward"
````

````

### **10. Notes Section**

```markdown
## 📝 Notes

- **Image Availability**: Ensure `[service-name]:latest` is loaded into your kind cluster using `kind load docker-image [service-name]:latest --name <cluster-name>`
- **Dependencies**: The application expects external services ([list dependencies]) - these may need to be deployed separately
- **Monitoring**: [Monitoring requirements and limitations]
- **Resource Requirements**: Adjust resource limits based on your local machine capabilities
- **[Service-Specific Notes]**: [Any special considerations for this service]
- **Architecture Compatibility**: The manifests are configured for ARM64 (Apple Silicon). For Intel Macs, change `nodeSelector` to `amd64`

---

Happy Kubernetes exploring! 🚀
````

## 🎨 **Writing Style Guidelines**

### **Tone & Voice**

- **Educational**: Explain the "why" behind each action
- **Encouraging**: Use positive language and success indicators
- **Practical**: Focus on actionable commands and real outcomes
- **Professional but Friendly**: Use emojis judiciously, maintain technical accuracy

### **Command Documentation**

- **Always include context**: Brief explanation of what each command does
- **Show expected outputs**: When helpful, include expected command outputs
- **Number commands**: Use `# 1.`, `# 2.` format for clarity
- **Group related commands**: Logical groupings with clear separations

### **Visual Elements**

- **Consistent Icons**: 🔐 secrets, ⚙️ config, 🚀 apps, 📊 monitoring, 🧪 testing
- **Status Indicators**: ✅ success, ❌ failure, ⚠️ warnings, 📝 notes
- **Code Blocks**: Always specify language for syntax highlighting
- **Sections**: Use clear hierarchical structure with descriptive headers

## 🔄 **Adaptation Guidelines**

### **For Different Service Types**

#### **API Services**

- Emphasize endpoint testing and health checks
- Include load balancing and scaling considerations
- Focus on request/response validation

#### **Worker Services**

- Emphasize queue connectivity and message processing
- Include worker scaling and task distribution
- Focus on job processing and error handling

#### **Database Services**

- Emphasize data persistence and backup
- Include connection testing and performance
- Focus on data integrity and recovery

#### **Gateway Services**

- Emphasize routing and upstream connectivity
- Include SSL/TLS and security considerations
- Focus on traffic management and monitoring

### **For Different Architectures**

#### **Event-Driven Services**

- Include event publishing/consuming tests
- Emphasize asynchronous processing validation
- Focus on event ordering and delivery guarantees

#### **Multi-Tenant Services**

- Include tenant isolation validation
- Emphasize security boundary testing
- Focus on resource segregation verification

## 🏗️ **Template Checklist**

When creating a new step-by-step guide, ensure:

### **Content Completeness** ✅

- [ ] Service introduction and architecture explanation
- [ ] Both automated and manual deployment options
- [ ] Kind-optimized deployment commands
- [ ] Comprehensive observation commands for each step
- [ ] Troubleshooting section with common issues
- [ ] Service-specific test suite
- [ ] Production deployment references

### **Technical Accuracy** ✅

- [ ] All commands tested on Kind cluster
- [ ] Resource names and labels consistent
- [ ] Expected outputs match reality
- [ ] Troubleshooting solutions verified
- [ ] Dependencies correctly identified

### **Educational Value** ✅

- [ ] Each step explains "what" and "why"
- [ ] Multiple observation angles provided
- [ ] Common failure modes addressed
- [ ] Learning progression from basic to advanced
- [ ] Cross-references to related concepts

### **Maintenance Considerations** ✅

- [ ] Version-agnostic commands where possible
- [ ] Clear indicators of version-specific content
- [ ] Easy-to-update configuration examples
- [ ] Links to canonical documentation sources

## 🚀 **Getting Started**

To create a new step-by-step deployment guide:

1. **Copy this template** and rename to `STEP_BY_STEP_DEPLOYMENT_GUIDE.md`
2. **Replace all `[service-name]` placeholders** with your service name
3. **Customize each section** based on your service's specific needs
4. **Test all commands** on a real Kind cluster
5. **Review with the team** for completeness and accuracy
6. **Update as the service evolves**

## 📚 **Reference Implementation**

See `deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md` in the profile-service for a complete implementation of these guidelines.

---

**Happy Guide Writing!** 📖✨
