# Enclii Infrastructure - Terraform Configuration
# Manages Hetzner compute, Cloudflare networking, and Ubicloud PostgreSQL

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.45"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.20"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }

  # Remote state storage - configure for your backend
  # Option 1: Cloudflare R2 (S3-compatible)
  backend "s3" {
    bucket                      = "enclii-terraform-state"
    key                         = "production/terraform.tfstate"
    region                      = "auto"
    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_s3_checksum            = true
    # endpoints configured via environment:
    # AWS_ENDPOINT_URL_S3=https://<account_id>.r2.cloudflarestorage.com
  }
}

# =============================================================================
# PROVIDERS
# =============================================================================

provider "hcloud" {
  token = var.hetzner_token
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

# =============================================================================
# DATA SOURCES
# =============================================================================

data "cloudflare_zone" "main" {
  name = var.domain
}

# =============================================================================
# NETWORKING - Hetzner Private Network
# =============================================================================

resource "hcloud_network" "enclii" {
  name     = "enclii-network"
  ip_range = "10.0.0.0/16"
}

resource "hcloud_network_subnet" "k8s" {
  network_id   = hcloud_network.enclii.id
  type         = "cloud"
  network_zone = "eu-central"
  ip_range     = "10.0.1.0/24"
}

resource "hcloud_network_subnet" "database" {
  network_id   = hcloud_network.enclii.id
  type         = "cloud"
  network_zone = "eu-central"
  ip_range     = "10.0.2.0/24"
}

# =============================================================================
# FIREWALL
# =============================================================================

resource "hcloud_firewall" "k8s_nodes" {
  name = "enclii-k8s-firewall"

  # SSH access (restricted to management IPs)
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = var.management_ips
  }

  # Kubernetes API
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "6443"
    source_ips = var.management_ips
  }

  # Allow all internal network traffic
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "any"
    source_ips = ["10.0.0.0/16"]
  }

  rule {
    direction  = "in"
    protocol   = "udp"
    port       = "any"
    source_ips = ["10.0.0.0/16"]
  }

  # Allow Cloudflare IPs for tunnel (egress only, no ingress needed)
  # cloudflared establishes outbound connections

  # Allow all outbound
  rule {
    direction       = "out"
    protocol        = "tcp"
    port            = "any"
    destination_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction       = "out"
    protocol        = "udp"
    port            = "any"
    destination_ips = ["0.0.0.0/0", "::/0"]
  }

  rule {
    direction       = "out"
    protocol        = "icmp"
    destination_ips = ["0.0.0.0/0", "::/0"]
  }
}

# =============================================================================
# SSH KEY
# =============================================================================

resource "tls_private_key" "ssh" {
  algorithm = "ED25519"
}

resource "hcloud_ssh_key" "enclii" {
  name       = "enclii-terraform"
  public_key = tls_private_key.ssh.public_key_openssh
}

# =============================================================================
# K3S CONTROL PLANE
# =============================================================================

resource "hcloud_server" "control_plane" {
  count       = var.control_plane_count
  name        = "enclii-cp-${count.index + 1}"
  server_type = var.control_plane_type
  image       = "ubuntu-22.04"
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.enclii.id]
  firewall_ids = [hcloud_firewall.k8s_nodes.id]

  labels = {
    role        = "control-plane"
    environment = "production"
    managed_by  = "terraform"
  }

  network {
    network_id = hcloud_network.enclii.id
    ip         = "10.0.1.${10 + count.index}"
  }

  user_data = templatefile("${path.module}/templates/k3s-server.yaml", {
    k3s_token       = random_password.k3s_token.result
    is_first_server = count.index == 0
    first_server_ip = "10.0.1.10"
    node_name       = "enclii-cp-${count.index + 1}"
    cluster_cidr    = "10.42.0.0/16"
    service_cidr    = "10.43.0.0/16"
  })

  depends_on = [hcloud_network_subnet.k8s]
}

# =============================================================================
# K3S WORKER NODES
# =============================================================================

resource "hcloud_server" "workers" {
  count       = var.worker_count
  name        = "enclii-worker-${count.index + 1}"
  server_type = var.worker_type
  image       = "ubuntu-22.04"
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.enclii.id]
  firewall_ids = [hcloud_firewall.k8s_nodes.id]

  labels = {
    role        = "worker"
    environment = "production"
    managed_by  = "terraform"
  }

  network {
    network_id = hcloud_network.enclii.id
    ip         = "10.0.1.${20 + count.index}"
  }

  user_data = templatefile("${path.module}/templates/k3s-agent.yaml", {
    k3s_token       = random_password.k3s_token.result
    server_ip       = "10.0.1.10"
    node_name       = "enclii-worker-${count.index + 1}"
  })

  depends_on = [
    hcloud_network_subnet.k8s,
    hcloud_server.control_plane
  ]
}

# =============================================================================
# SECRETS
# =============================================================================

resource "random_password" "k3s_token" {
  length  = 64
  special = false
}

resource "random_password" "postgres_password" {
  length  = 32
  special = true
}

resource "random_password" "redis_password" {
  length  = 32
  special = false
}

# =============================================================================
# VOLUMES
# =============================================================================

resource "hcloud_volume" "postgres_data" {
  name      = "enclii-postgres-data"
  size      = var.postgres_volume_size
  location  = var.location
  format    = "ext4"

  labels = {
    purpose     = "database"
    environment = "production"
    managed_by  = "terraform"
  }
}

resource "hcloud_volume_attachment" "postgres_data" {
  volume_id = hcloud_volume.postgres_data.id
  server_id = hcloud_server.workers[0].id
  automount = true
}

# =============================================================================
# OUTPUTS
# =============================================================================

output "control_plane_ips" {
  description = "Control plane node IPs"
  value = {
    public  = hcloud_server.control_plane[*].ipv4_address
    private = [for s in hcloud_server.control_plane : s.network[0].ip]
  }
}

output "worker_ips" {
  description = "Worker node IPs"
  value = {
    public  = hcloud_server.workers[*].ipv4_address
    private = [for s in hcloud_server.workers : s.network[0].ip]
  }
}

output "ssh_private_key" {
  description = "SSH private key for node access"
  value       = tls_private_key.ssh.private_key_openssh
  sensitive   = true
}

output "k3s_token" {
  description = "K3s cluster join token"
  value       = random_password.k3s_token.result
  sensitive   = true
}

output "postgres_password" {
  description = "PostgreSQL password"
  value       = random_password.postgres_password.result
  sensitive   = true
}

output "redis_password" {
  description = "Redis password"
  value       = random_password.redis_password.result
  sensitive   = true
}
