variable "hcloud_token" {
  type      = string
  sensitive = true
}

variable "ssh_allowed_cidr" {
  type        = string
  description = "Your public IP in CIDR form (e.g. 1.2.3.4/32)"
}

variable "ssh_public_key_path" {
  type        = string
  description = "Path to your SSH public key"
  default     = "~/.ssh/id_ed25519.pub"
}

# Hetzner location (e.g. hel1, fsn1, nbg1, ash, hil)
variable "location" {
  type    = string
  default = "ash"
}

# Hetzner network zone: eu-central | us-east | us-west | ap-southeast
variable "network_zone" {
  type    = string
  default = "us-east"
}

variable "image" {
  type    = string
  default = "ubuntu-24.04"
}

# Sizing
variable "controlplane_type" {
  type    = string
  default = "cpx31" # 2 vCPU / 8 GB
}

variable "worker_type" {
  type    = string
  default = "cpx21" # 2 vCPU / 4 GB
}

# Private networking
variable "network_cidr" {
  type    = string
  default = "10.10.0.0/16"
}

variable "subnet_cidr" {
  type    = string
  default = "10.10.1.0/24"
}

# Naming
variable "project_name" {
  type    = string
  default = "nimbul"
}

variable "env" {
  type    = string
  default = "dev"
}
