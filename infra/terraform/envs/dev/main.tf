terraform {
  required_version = ">= 1.5.0"
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.48"
    }
  }
}

provider "hcloud" {
  token = var.hcloud_token
}

locals {
  name_prefix = "${var.project_name}-${var.env}"
}

resource "hcloud_ssh_key" "this" {
  name       = "${local.name_prefix}-key"
  public_key = file(pathexpand(var.ssh_public_key_path))
}

resource "hcloud_network" "this" {
  name     = "${local.name_prefix}-net"
  ip_range = var.network_cidr
}

resource "hcloud_network_subnet" "this" {
  network_id   = hcloud_network.this.id
  type         = "cloud"
  network_zone = var.network_zone
  ip_range     = var.subnet_cidr
}

resource "hcloud_firewall" "this" {
  name = "${local.name_prefix}-fw"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = [var.ssh_allowed_cidr]
  }

  # HTTP/HTTPS for ingress later
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "controlplane" {
  name         = "${local.name_prefix}-cp-1"
  server_type  = var.controlplane_type
  image        = var.image
  location     = var.location
  ssh_keys     = [hcloud_ssh_key.this.id]
  firewall_ids = [hcloud_firewall.this.id]
}

resource "hcloud_server" "worker" {
  count        = 2
  name         = "${local.name_prefix}-worker-${count.index + 1}"
  server_type  = var.worker_type
  image        = var.image
  location     = var.location
  ssh_keys     = [hcloud_ssh_key.this.id]
  firewall_ids = [hcloud_firewall.this.id]
}

resource "hcloud_server_network" "cp_net" {
  server_id  = hcloud_server.controlplane.id
  network_id = hcloud_network.this.id
}

resource "hcloud_server_network" "worker_net" {
  count      = 2
  server_id  = hcloud_server.worker[count.index].id
  network_id = hcloud_network.this.id
}
