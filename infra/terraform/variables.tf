# Enclii Infrastructure Variables

# =============================================================================
# PROVIDER CREDENTIALS
# =============================================================================

variable "hetzner_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "cloudflare_api_token" {
  description = "Cloudflare API token with Zone and Tunnel permissions"
  type        = string
  sensitive   = true
}

variable "cloudflare_account_id" {
  description = "Cloudflare account ID"
  type        = string
}

# =============================================================================
# DOMAIN & DNS
# =============================================================================

variable "domain" {
  description = "Primary domain for Enclii (e.g., enclii.dev)"
  type        = string
  default     = "enclii.dev"
}

variable "subdomain_api" {
  description = "Subdomain for API"
  type        = string
  default     = "api"
}

variable "subdomain_app" {
  description = "Subdomain for web app"
  type        = string
  default     = "app"
}

# =============================================================================
# HETZNER CONFIGURATION
# =============================================================================

variable "location" {
  description = "Hetzner datacenter location"
  type        = string
  default     = "nbg1" # Nuremberg, Germany

  validation {
    condition     = contains(["nbg1", "fsn1", "hel1", "ash", "hil"], var.location)
    error_message = "Location must be a valid Hetzner datacenter: nbg1, fsn1, hel1, ash, hil"
  }
}

variable "control_plane_count" {
  description = "Number of control plane nodes (should be odd: 1, 3, or 5)"
  type        = number
  default     = 1

  validation {
    condition     = contains([1, 3, 5], var.control_plane_count)
    error_message = "Control plane count must be 1, 3, or 5 for HA"
  }
}

variable "control_plane_type" {
  description = "Server type for control plane nodes"
  type        = string
  default     = "cx21" # 2 vCPU, 4GB RAM, €5.18/mo
}

variable "worker_count" {
  description = "Number of worker nodes"
  type        = number
  default     = 2

  validation {
    condition     = var.worker_count >= 1 && var.worker_count <= 10
    error_message = "Worker count must be between 1 and 10"
  }
}

variable "worker_type" {
  description = "Server type for worker nodes"
  type        = string
  default     = "cx31" # 2 vCPU, 8GB RAM, €9.92/mo
}

variable "postgres_volume_size" {
  description = "Size of PostgreSQL data volume in GB"
  type        = number
  default     = 50

  validation {
    condition     = var.postgres_volume_size >= 10 && var.postgres_volume_size <= 1000
    error_message = "Volume size must be between 10 and 1000 GB"
  }
}

# =============================================================================
# NETWORK SECURITY
# =============================================================================

variable "management_ips" {
  description = "IP addresses allowed for SSH and Kubernetes API access"
  type        = list(string)
  default     = []

  # Example: ["203.0.113.0/24", "198.51.100.50/32"]
}

# =============================================================================
# R2 STORAGE
# =============================================================================

variable "r2_access_key_id" {
  description = "Cloudflare R2 access key ID"
  type        = string
  sensitive   = true
}

variable "r2_secret_access_key" {
  description = "Cloudflare R2 secret access key"
  type        = string
  sensitive   = true
}

variable "r2_bucket_backups" {
  description = "R2 bucket name for backups"
  type        = string
  default     = "enclii-backups"
}

variable "r2_bucket_artifacts" {
  description = "R2 bucket name for build artifacts"
  type        = string
  default     = "enclii-artifacts"
}

# =============================================================================
# ENVIRONMENT
# =============================================================================

variable "environment" {
  description = "Deployment environment"
  type        = string
  default     = "production"

  validation {
    condition     = contains(["development", "staging", "production"], var.environment)
    error_message = "Environment must be development, staging, or production"
  }
}

variable "tags" {
  description = "Common tags for all resources"
  type        = map(string)
  default = {
    project    = "enclii"
    managed_by = "terraform"
  }
}
