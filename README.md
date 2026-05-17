# k8s-mtp

A Kubernetes multi-tenant platform that provides isolated tenant environments with resource quotas, RBAC management, network policies, security enforcement through admission webhooks, a REST API with JWT authentication, and a CLI management tool.

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

### Authentication & API
- **Dex OIDC**: Integrated identity provider with static passwords for bootstrapping (Helm-managed)
- **JWT Middleware**: Token validation with JWKS key rotation, issuer and audience enforcement
- **REST API**: Full tenant and member CRUD (8 endpoints) with authenticated access
- **Rate Limiting**: Per-user token bucket rate limiter with configurable RPM and burst
- **Member Management**: Add, remove, and list tenant members with admin/operator roles
- **CLI Tool**: `k8s-mtp` binary for login, tenant CRUD, and member management
- **Configurable Auth**: Toggle authentication on/off for development (`auth_enabled: false`)

### Resource Management
- **ResourceQuotas**: Tier-based limits for CPU, Memory, Pods, and Storage
- **LimitRanges**: Default resource requests and limits per tier
- **Configurable Defaults**: Tier-specific LimitRange defaults via configuration

## Architecture

The platform consists of six main components:

1. **Controller** (`cmd/controller`): 
   - Watches Tenant CRDs using controller-runtime reconciler pattern
   - Manages namespace lifecycle (creation, updates, deletion with finalizers)
   - Creates and manages NetworkPolicies, RBAC, ResourceQuotas, and LimitRanges
   - Self-healing: recreates resources if manually deleted

2. **API Server** (`cmd/api`): 
   - REST API for tenant and member CRUD operations (8 endpoints)
   - JWT validation middleware fetches JWKS from Dex and enforces issuer/audience
   - Per-user rate limiting with configurable RPM and burst
   - Structured logging with slog
   - Middleware chain: logging → rate limit → auth → handler

3. **Webhook Server** (`cmd/webhook`): 
   - Validating admission controller for pod security enforcement
   - Mutating admission controller for automatic tenant labeling
   - TLS termination with certificate validation

4. **Webhook Manager** (`internal/webhook`):
   - Deploys and manages webhook infrastructure
   - Handles TLS certificate generation and rotation
   - Manages ValidatingWebhookConfiguration and MutatingWebhookConfiguration

5. **Dex** (OIDC Provider):
   - Manages authentication via static passwords (bootstrapping), extensible to external IdPs
   - Issues signed JWTs consumed by the API server's auth middleware
   - Helm-managed deployment with SQLite storage

6. **CLI Tool** (`cmd/cli/`):
   - `k8s-mtp login` — exchanges Dex credentials for a JWT
   - `k8s-mtp tenant` — list, create, get, update, delete tenants via the REST API
   - `k8s-mtp member` — add, list, remove tenant members
   - Stores config and token in `~/.k8s-mtp/config`

## Quick Start

### Prerequisites
- Kubernetes cluster (K3s recommended) v1.28+
- PostgreSQL database
- kubectl configured
- Helm 3

### Deployment

```bash
# 1. Deploy the CRD
kubectl apply -f config/crds/multitenant.k8s-mtp.io_tenants.yaml

# 2. Install the platform via Helm
helm install k8s-mtp deploy/charts/k8s-mtp \
  --set db.password=<your-postgres-password> \
  --set image.tag=latest \
  --set ingress.host=api.k8s-mtp.local \
  --set dex.enabled=true
```

The Helm chart deploys: namespace, ConfigMap, Secret, API Deployment and Service, Controller Deployment with RBAC, Dex Deployment/Service/Secret (optional), and optional Ingress.

To provision the underlying infrastructure (K3s cluster + PostgreSQL), see the Terraform configs in `deploy/terraform/`.

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

### Using the CLI

```bash
# Build the CLI
make build

# Configure the CLI (set your Dex and API URLs first)
mkdir -p ~/.k8s-mtp
echo '{"dex_url":"http://localhost:5556","api_url":"http://localhost:8080","client_id":"k8s-mtp-cli"}' > ~/.k8s-mtp/config

# Login (uses Dex password grant)
./bin/k8s-mtp login
# Username: admin
# Password: <static password from Helm deployment>

# Manage tenants
./bin/k8s-mtp tenant list
./bin/k8s-mtp tenant create '{"name":"acme","tier":"free","owner_email":"admin@acme.com"}'
./bin/k8s-mtp tenant get <tenant-id>

# Manage members
./bin/k8s-mtp member add <tenant-id> <user-id> admin
./bin/k8s-mtp member list <tenant-id>
./bin/k8s-mtp member remove <tenant-id> <user-id>
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
│   ├── cli/              # CLI management tool
│   ├── controller/       # K8s operator with reconciler
│   └── webhook/          # Admission webhook server
├── internal/
│   ├── config/           # Configuration parsing
│   ├── handlers/         # REST API handlers (tenants + members)
│   ├── middleware/        # JWT authentication + rate limiting
│   ├── reconciler/       # Controller reconciliation logic
│   ├── server/           # HTTP server setup and logging
│   ├── store/            # Database connection, migrations, CRUD
│   │   └── migrations/   # Embedded SQL schema files
│   └── webhook/          # Webhook deployment manager
├── pkg/
│   ├── api/              # API types (Tenant CRD)
│   └── webhook/          # TLS certificate generation
├── base/                 # Custom FROM scratch base image
├── config/
│   └── crds/             # Kubernetes CRDs
├── deploy/
│   ├── charts/k8s-mtp/   # Helm chart (incl. Dex templates)
│   │   └── templates/
│   │       ├── dex-secret.yaml
│   │       ├── dex-deployment.yaml
│   │       └── dex-service.yaml
│   └── terraform/        # K3s + PostgreSQL provisioning
├── .gitea/workflows/     # Gitea Actions CI pipeline
├── Makefile
├── .ko.yaml              # Ko build config
└── config.json           # Default runtime config
```

### Running Tests

```bash
# Run unit tests
make test

# Run linting
make lint

# Build binaries
make build
```

To test on a live cluster:
```bash
# Install the platform
helm install k8s-mtp deploy/charts/k8s-mtp --set db.password=<password>

# Create a tenant (see Quick Start for the Tenant YAML example)
kubectl apply -f tenant.yaml

# Verify resources created
kubectl get all -n tenant-<tenant-name>-<spec-name>
kubectl get networkpolicy -n tenant-<tenant-name>-<spec-name>
kubectl get roles,rolebindings -n tenant-<tenant-name>-<spec-name>
```

### CI/CD

On push to `main`, the Gitea Actions pipeline (`.gitea/workflows/ci.yaml`) runs:

1. **Lint** (`go vet`)
2. **Test** (`go test ./...`)
3. **Build & push** container images via ko (daemonless, self-hosted)
4. **Package & push** Helm chart to Gitea registry

### Technology Stack

- **Go 1.25+** - Standard library focus (net/http, slog, database/sql)
- **controller-runtime** - Kubernetes operator framework
- **golang-jwt/jwt/v5** - JWT parsing and validation
- **Dex** - OIDC identity provider (Helm-managed, optional)
- **K3s** - Lightweight Kubernetes distribution
- **PostgreSQL** - Tenant metadata storage
- **ko** - Daemonless Go container builds
- **golang-migrate** - Embedded database migrations
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
- [Part 4: Middleware](https://excipio.tech/blog/k8s-mtp-a-multi-tenant-kubernetes-platform-pt.-4/)

## License

MIT License - see [LICENSE](LICENSE)
