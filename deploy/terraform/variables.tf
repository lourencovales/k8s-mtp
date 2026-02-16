variable "vps_ip" {
  description = "Public IP address of the VPS"
  type        = string
}

variable "ssh_username" {
  description = "SSH username for VPS access"
  type        = string
}

variable "ssh_port" {
  description = "SSH port"
  type        = number
  default     = 22
}

variable "use_ssh_agent" {
  description = "Use SSH agent for authentication"
  type        = bool
  default     = true
}

variable "k3s_version" {
  description = "K3s version to install"
  type        = string
  default     = "v1.29.0+k3s1"
}

variable "postgres_namespace" {
  description = "Kubernetes namespace for PostgreSQL"
  type        = string
  default     = "k8s-mtp"
}

variable "postgres_storage_size" {
  description = "PostgreSQL storage size"
  type        = string
  default     = "10Gi"
}

variable "postgres_password" {
  description = "PostgreSQL password"
  type        = string
  sensitive   = true
}

variable "app_namespace" {
  description = "Kubernetes namespace for the application"
  type        = string
  default     = "k8s-mtp"
}
