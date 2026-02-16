# K3s Installation on VPS
resource "null_resource" "k3s_install" {
  triggers = {
    always_run = timestamp()
  }

  connection {
    type  = "ssh"
    host  = var.vps_ip
    user  = var.ssh_username
    port  = var.ssh_port
    agent = var.use_ssh_agent
  }

  # Update system and install prerequisites
  provisioner "remote-exec" {
    inline = [
      "sudo apt-get update",
      "sudo apt-get install -y curl wget",
    ]
  }

  # Install K3s
  provisioner "remote-exec" {
    inline = [
      "curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=${var.k3s_version} sh -s - server --cluster-init",
      "sleep 10",
      "sudo systemctl is-active --quiet k3s || exit 1",
    ]
  }
}

# Setup kubectl configuration for the user
resource "null_resource" "setup_kubectl" {
  depends_on = [null_resource.k3s_install]

  triggers = {
    always_run = timestamp()
  }

  connection {
    type  = "ssh"
    host  = var.vps_ip
    user  = var.ssh_username
    port  = var.ssh_port
    agent = var.use_ssh_agent
  }

  provisioner "remote-exec" {
    inline = [
      # Setup kubectl config for the user
      "mkdir -p ~/.kube",
      "sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config",
      "sudo chown $(id -u):$(id -g) ~/.kube/config",
      "chmod 600 ~/.kube/config",
      "echo 'export KUBECONFIG=~/.kube/config' >> ~/.bashrc"
    ]
  }
}

# Fetch kubeconfig locally
resource "null_resource" "fetch_kubeconfig" {
  depends_on = [null_resource.setup_kubectl]

  triggers = {
    always_run = timestamp()
  }

  provisioner "local-exec" {
    command = <<-EOT
      mkdir -p ~/.kube
      ssh -p ${var.ssh_port} ${var.ssh_username}@${var.vps_ip} "cat ~/.kube/config" > ~/.kube/k8s-mtp-config
      sed -i 's/127.0.0.1/${var.vps_ip}/g' ~/.kube/k8s-mtp-config
      chmod 600 ~/.kube/k8s-mtp-config
      echo "Kubeconfig saved to ~/.kube/k8s-mtp-config"
    EOT
  }
}

# Template and copy manifest files
locals {
  namespace_manifest = templatefile("${path.module}/manifests/01-namespace.yaml.tpl", {
    namespace = var.postgres_namespace
  })
  
  postgres_manifest = templatefile("${path.module}/manifests/02-postgres.yaml.tpl", {
    namespace     = var.postgres_namespace
    storage_size  = var.postgres_storage_size
    password      = var.postgres_password
  })
  
  app_namespace_manifest = templatefile("${path.module}/manifests/03-app-namespace.yaml.tpl", {
    namespace = var.app_namespace
  })
}

resource "null_resource" "copy_manifests" {
  depends_on = [null_resource.setup_kubectl]

  triggers = {
    always_run = timestamp()
  }

  connection {
    type  = "ssh"
    host  = var.vps_ip
    user  = var.ssh_username
    port  = var.ssh_port
    agent = var.use_ssh_agent
  }

  # Create manifests directory and write files
  provisioner "remote-exec" {
    inline = [
      "mkdir -p ~/k8s-mtp-manifests",
      "cat > ~/k8s-mtp-manifests/01-namespace.yaml << 'EOFEOFEOF'",
      local.namespace_manifest,
      "EOFEOFEOF",
      "cat > ~/k8s-mtp-manifests/02-postgres.yaml << 'EOFEOFEOF'",
      local.postgres_manifest,
      "EOFEOFEOF",
      "cat > ~/k8s-mtp-manifests/03-app-namespace.yaml << 'EOFEOFEOF'",
      local.app_namespace_manifest,
      "EOFEOFEOF",
      "echo 'Manifests copied successfully'",
    ]
  }
}

# Apply manifests
resource "null_resource" "apply_manifests" {
  depends_on = [null_resource.copy_manifests]

  triggers = {
    always_run = timestamp()
  }

  connection {
    type  = "ssh"
    host  = var.vps_ip
    user  = var.ssh_username
    port  = var.ssh_port
    agent = var.use_ssh_agent
  }

  provisioner "remote-exec" {
    inline = [
      "KUBECONFIG=~/.kube/config kubectl apply -f ~/k8s-mtp-manifests/",
      "sleep 5",
      "KUBECONFIG=~/.kube/config kubectl wait --for=condition=ready pod -l app=postgres -n ${var.postgres_namespace} --timeout=120s",
      "echo 'All manifests applied successfully'",
    ]
  }
}
