# Hetzner-specific Configuration for Enclii
# Additional Hetzner resources and configurations

# =============================================================================
# LOAD BALANCER (Optional - for non-tunnel setups)
# =============================================================================

# Uncomment if you need a traditional load balancer instead of Cloudflare Tunnel
# resource "hcloud_load_balancer" "ingress" {
#   name               = "enclii-ingress"
#   load_balancer_type = "lb11"
#   location           = var.location
#
#   labels = {
#     purpose     = "ingress"
#     environment = var.environment
#     managed_by  = "terraform"
#   }
# }
#
# resource "hcloud_load_balancer_network" "ingress" {
#   load_balancer_id = hcloud_load_balancer.ingress.id
#   network_id       = hcloud_network.enclii.id
#   ip               = "10.0.1.2"
# }
#
# resource "hcloud_load_balancer_target" "workers" {
#   count            = var.worker_count
#   load_balancer_id = hcloud_load_balancer.ingress.id
#   type             = "server"
#   server_id        = hcloud_server.workers[count.index].id
#   use_private_ip   = true
# }

# =============================================================================
# PLACEMENT GROUPS (Spread servers across fault domains)
# =============================================================================

resource "hcloud_placement_group" "control_plane" {
  name = "enclii-cp-spread"
  type = "spread"

  labels = {
    role        = "control-plane"
    environment = var.environment
  }
}

resource "hcloud_placement_group" "workers" {
  name = "enclii-worker-spread"
  type = "spread"

  labels = {
    role        = "worker"
    environment = var.environment
  }
}

# =============================================================================
# FLOATING IPS (Optional - for stable egress)
# =============================================================================

resource "hcloud_floating_ip" "egress" {
  count         = var.enable_floating_ip ? 1 : 0
  type          = "ipv4"
  home_location = var.location

  labels = {
    purpose     = "egress"
    environment = var.environment
    managed_by  = "terraform"
  }
}

resource "hcloud_floating_ip_assignment" "egress" {
  count          = var.enable_floating_ip ? 1 : 0
  floating_ip_id = hcloud_floating_ip.egress[0].id
  server_id      = hcloud_server.control_plane[0].id
}

# =============================================================================
# ADDITIONAL VOLUMES
# =============================================================================

# Build cache volume for faster builds
resource "hcloud_volume" "build_cache" {
  name      = "enclii-build-cache"
  size      = var.build_cache_volume_size
  location  = var.location
  format    = "ext4"

  labels = {
    purpose     = "build-cache"
    environment = var.environment
    managed_by  = "terraform"
  }
}

# Redis persistence volume (if not using managed Redis)
resource "hcloud_volume" "redis_data" {
  name      = "enclii-redis-data"
  size      = var.redis_volume_size
  location  = var.location
  format    = "ext4"

  labels = {
    purpose     = "cache"
    environment = var.environment
    managed_by  = "terraform"
  }
}

# =============================================================================
# ADDITIONAL VARIABLES
# =============================================================================

variable "enable_floating_ip" {
  description = "Enable floating IP for stable egress"
  type        = bool
  default     = false
}

variable "build_cache_volume_size" {
  description = "Size of build cache volume in GB"
  type        = number
  default     = 100
}

variable "redis_volume_size" {
  description = "Size of Redis data volume in GB"
  type        = number
  default     = 10
}

# =============================================================================
# OUTPUTS
# =============================================================================

output "floating_ip" {
  description = "Floating IP for egress (if enabled)"
  value       = var.enable_floating_ip ? hcloud_floating_ip.egress[0].ip_address : null
}

output "network_id" {
  description = "Hetzner private network ID"
  value       = hcloud_network.enclii.id
}

output "placement_group_cp_id" {
  description = "Control plane placement group ID"
  value       = hcloud_placement_group.control_plane.id
}

output "placement_group_worker_id" {
  description = "Worker placement group ID"
  value       = hcloud_placement_group.workers.id
}
