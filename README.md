# k8s-mtp

A Kubernetes multi-tenant platform that provides isolated tenant environments with resource quotas, RBAC management, and tier-based resource allocation.

## Overview

k8s-mtp (Kubernetes Multi-Tenant Platform) enables cluster administrators to create and manage isolated tenant environments within a Kubernetes cluster. Each tenant gets their own namespace with predefined resource quotas and role-based access control.

## Features

- **Tenant CRD**: Custom resource definition for managing tenant lifecycles
- **Three-Tier System**: Free, Pro, and Enterprise tiers with different resource quotas
- **Automatic Resource Provisioning**: Creates namespaces, ResourceQuotas, and RBAC resources
- **Webhook Validation**: Pod admission control with TLS-secured webhooks
- **REST API**: HTTP API for tenant management operations
- **PostgreSQL Backend**: Persistent storage for tenant metadata

## Architecture

The platform consists of three main components:

1. **Controller** (`cmd/controller`): Watches Tenant CRDs and reconciles cluster resources
2. **API Server** (`cmd/api`): REST API for external integrations
3. **Webhook** (`cmd/webhook`): Admission controller for pod validation

## Tenant Tiers

| Tier | CPU | Memory | Pods | Storage |
|------|-----|--------|------|---------|
| Free | 500m | 1Gi | 20 | 10Gi |
| Pro | 4 | 8Gi | 100 | 100Gi |
| Enterprise | 8 | 16Gi | 200 | 200Gi |

## Quick Start

```bash
# Deploy the CRDs
kubectl apply -f config/crds/

# Deploy with Terraform
cd deploy/terraform
terraform init
terraform apply
```

## Example Tenant

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

## License

MIT License - see [LICENSE](LICENSE)
