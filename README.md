# k8s-mtp

A Kubernetes multi-tenant platform that provides isolated tenant environments with resource quotas, RBAC management, network policies, and comprehensive security enforcement through admission webhooks.

## Overview

k8s-mtp (Kubernetes Multi-Tenant Platform) enables cluster administrators to create and manage isolated tenant environments within a Kubernetes cluster. Each tenant gets their own namespace with predefined resource quotas, role-based access control, network isolation, and enforced security standards.

Built with Go's standard library where possible, minimizing external dependencies while maintaining production-grade security and reliability.

## Features

### Core Platform
- **Tenant CRD**: Custom resource definition for managing tenant lifecycles (cluster-scoped)
- **Three-Tier System**: Free, Pro, and Enterprise tiers with different resource quotas
- **Automatic Resource Provisioning**: Creates namespaces, ResourceQuotas, RBAC resources, NetworkPolicies, and LimitRanges
- **PostgreSQL Backend**: Persistent storage for tenant metadata with migrations support

### Security Layer
- **Admission Webhooks**: 
  - Validating webhook enforces pod security standards
  - Mutating webhook automatically adds tenant labels for resource tracking
  - TLS-secured with self-signed certificate management
- **Network Isolation**: NetworkPolicies block cross-tenant traffic by default
- **Pod Security Standards**: Enforces:
  - Non-root containers (runAsNonRoot: true)
  - Read-only root filesystem
  - No privilege escalation
  - No privileged containers
  - Required resource limits (CPU and Memory)
- **RBAC Management**: Automated Role and RoleBinding creation with admin/operator differentiation

### Resource Management
- **ResourceQuotas**: Tier-based limits for CPU, Memory, Pods, and Storage
- **LimitRanges**: Default resource requests and limits per tier
- **Configurable Defaults**: Tier-specific LimitRange defaults via configuration

## Architecture

The platform consists of four main components:

1. **Controller** (`cmd/controller`): 
   - Watches Tenant CRDs using controller-runtime reconciler pattern
   - Manages namespace lifecycle (creation, updates, deletion with finalizers)
   - Creates and manages NetworkPolicies, RBAC, ResourceQuotas, and LimitRanges
   - Self-healing: recreates resources if manually deleted

2. **API Server** (`cmd/api`): 
   - REST API for external integrations
   - Structured logging with slog
   - Middleware pattern for authentication and rate limiting

3. **Webhook Server** (`cmd/webhook`): 
   - Validating admission controller for pod security enforcement
   - Mutating admission controller for automatic tenant labeling
   - TLS termination with certificate validation

4. **Webhook Manager** (`internal/webhook`):
   - Deploys and manages webhook infrastructure
   - Handles TLS certificate generation and rotation
   - Manages ValidatingWebhookConfiguration and MutatingWebhookConfiguration

## Tenant Tiers

| Tier | CPU | Memory | Pods | Storage |
|------|-----|--------|------|---------|
| Free | 500m | 1Gi | 20 | 10Gi |
| Pro | 4 | 8Gi | 100 | 100Gi |
| Enterprise | 8 | 16Gi | 200 | 200Gi |

### Default Resource Limits

| Tier | Default CPU | Default Memory |
|------|-------------|----------------|
| Free | 100m | 128Mi |
| Pro | 200m | 256Mi |
| Enterprise | 500m | 512Mi |

## Quick Start

### Prerequisites
- Kubernetes cluster (K3s recommended) v1.28+
- PostgreSQL database
- kubectl configured

### Deployment

Deploy components in this order:

```bash
# 1. Deploy the CRD
kubectl apply -f config/crds/multitenant.k8s-mtp.io_tenants.yaml

# 2. Deploy RBAC resources
kubectl apply -f test/rbac-controller-sa.yaml
kubectl apply -f test/rbac-controller-crole.yaml
kubectl apply -f test/rbac-controller-crolebinding.yaml

# 3. Deploy ConfigMap and Secret
kubectl apply -f test/configmap.yaml
kubectl apply -f test/secret.yaml

# 4. Deploy API and Controller
kubectl apply -f test/deploymentl-api.yaml
kubectl apply -f test/deploymentl-controller.yaml

# 5. Deploy Ingress (optional)
kubectl apply -f test/ingress.yaml
```

### Creating a Tenant

```yaml
apiVersion: multitenant.k8s-mtp.io/v1
kind: Tenant
metadata:
  name: acme-corp
spec:
  name: acme
  tier: pro
  ownerEmail: admin@acme.com
  admins:
    - alice@acme.com
  operators:
    - bob@acme.com
```

Apply with:
```bash
kubectl apply -f tenant.yaml
```

### Troubleshooting

**kubectl cache issues after CRD changes:**
```bash
rm -rf ~/.kube/cache
```

**Webhook TLS certificate issues:**
Delete and regenerate:
```bash
kubectl delete validatingwebhookconfiguration k8s-mtp-webhook
kubectl delete mutatingwebhookconfiguration k8s-mtp-mutating-webhook
kubectl delete secret webhook-tls -n k8s-mtp
kubectl rollout restart deployment/controller -n k8s-mtp
```

## Security Enforcement

The platform enforces the following security policies through the validating webhook:

### Blocked Configurations

| Violation | Error Message |
|-----------|---------------|
| Privileged containers | "Container nginx failed privilege check" |
| Root user (runAsNonRoot: false) | "Container nginx failed privilege check" |
| Missing security context | "Container nginx must have SecurityContext defined" |
| Writable root filesystem | "Container nginx must have read only root filesystem" |
| Privilege escalation allowed | "Container nginx must not be allowed to escalate privileges" |
| Missing resource limits | "Container nginx must have resource limits" |
| Missing CPU limits | "Container nginx must have CPU limits" |
| Missing Memory limits | "Container nginx must have Memory limits" |

### Testing Security Policies

Create test pods to verify enforcement:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: privileged-pod
  namespace: tenant-acme-acme  # Replace with your tenant namespace
spec:
  containers:
  - name: nginx
    image: nginx
    resources:
      limits:
        cpu: "100m"
        memory: "128Mi"
    securityContext:
      privileged: true
      runAsNonRoot: true
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
```

This should be rejected with:
```
Error from server: admission webhook "k8s-mtp-webhook.k8s-mtp.io" denied the request: Container nginx failed privilege check
```

### Network Isolation

By default, tenants are isolated from each other:
- **Ingress**: Only allows traffic from pods within the same namespace and platform services
- **Egress**: Allows DNS (kube-system), same-namespace traffic, and platform services
- **Cross-tenant**: Blocked by NetworkPolicy
- **Internet**: Optional, controlled by `TenantEgressPolicy` configuration

## Development

### Project Structure

```
k8s-mtp/
├── cmd/
│   ├── api/              # HTTP API server
│   ├── controller/       # K8s operator with reconciler
│   └── webhook/          # Admission webhook server
├── internal/
│   ├── config/           # Configuration parsing
│   ├── reconciler/       # Controller reconciliation logic
│   ├── store/            # Database connection and migrations
│   └── webhook/          # Webhook deployment manager
├── pkg/
│   ├── api/              # API types (Tenant CRD)
│   └── webhook/          # TLS certificate generation
├── config/
│   └── crds/             # Kubernetes CRDs
└── test/                 # Test manifests
```

### Running Tests

```bash
# Create a test tenant
kubectl apply -f test/test-tenant.yaml

# Test security policies
kubectl apply -f test/test-tenant-reject.yaml

# Verify resources created
kubectl get all -n tenant-test-acme-acme
kubectl get networkpolicy -n tenant-test-acme-acme
kubectl get roles,rolebindings -n tenant-test-acme-acme
```

### Technology Stack

- **Go 1.21+** - Standard library focus (net/http, slog, database/sql)
- **controller-runtime** - Kubernetes operator framework
- **K3s** - Lightweight Kubernetes distribution
- **PostgreSQL** - Tenant metadata storage
- **lib/pq** - PostgreSQL driver (stdlib compatible)

### Design Principles

- **Minimal dependencies**: Use Go standard library where possible
- **Security by default**: Everything blocked unless explicitly allowed
- **Self-healing**: Controller reconciles drift automatically
- **Zero-trust**: Network isolation between tenants
- **Defense in depth**: Multiple security layers (webhook, network, RBAC)

## Blog Series

This project is documented in a blog series:
- [Part 1: Foundations](https://excipio.tech/blog/k8s-mtp-a-multi-tenant-kubernetes-platform-pt.-1/)
- [Part 2: Core Controller](https://excipio.tech/blog/k8s-mtp-a-multi-tenant-kubernetes-platform-pt.-2/)
- [Part 3: Security Layer](https://excipio.tech/blog/k8s-mtp-a-multi-tenant-kubernetes-platform-pt.-3/)

## License

MIT License - see [LICENSE](LICENSE)
