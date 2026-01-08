output "controlplane_public_ip" {
  value = hcloud_server.controlplane.ipv4_address
}

output "controlplane_private_ip" {
  value = hcloud_server_network.cp_net.ip
}

output "worker_public_ips" {
  value = [for s in hcloud_server.worker : s.ipv4_address]
}

output "worker_private_ips" {
  value = [for n in hcloud_server_network.worker_net : n.ip]
}

output "ansible_inventory" {
  value = <<-EOT
    [controlplane]
    ${hcloud_server.controlplane.ipv4_address} private_ip=${hcloud_server_network.cp_net.ip}

    [workers]
    ${hcloud_server.worker[0].ipv4_address} private_ip=${hcloud_server_network.worker_net[0].ip}
    ${hcloud_server.worker[1].ipv4_address} private_ip=${hcloud_server_network.worker_net[1].ip}

    [all:vars]
    ansible_user=root
    ansible_ssh_private_key_file=~/.ssh/id_ed25519
  EOT
}
