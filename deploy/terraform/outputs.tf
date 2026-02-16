output "vps_ip" {
  description = "VPS IP address"
  value       = var.vps_ip
}

output "kubeconfig_local_path" {
  description = "Local path to kubeconfig file"
  value       = "~/.kube/k8s-mtp-config"
}

output "kubectl_command" {
  description = "Command to use kubectl"
  value       = "export KUBECONFIG=~/.kube/k8s-mtp-config && kubectl get nodes"
}

output "postgres_connection_internal" {
  description = "PostgreSQL internal connection string"
  value       = "postgres://k8s-mtp:<password>@postgres.${var.postgres_namespace}.svc.cluster.local:5432/k8s-mtp"
  sensitive   = true
}

output "postgres_namespace" {
  description = "PostgreSQL namespace"
  value       = var.postgres_namespace
}

output "app_namespace" {
  description = "Application namespace"
  value       = var.app_namespace
}

output "deployment_status" {
  description = "Status of deployment"
  value       = "K3s, PostgreSQL, and application namespace deployed successfully"
}
