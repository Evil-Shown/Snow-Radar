output "server_public_ip" {
  value = hcloud_server.snow_radar.ipv4_address
}

output "server_id" {
  value = hcloud_server.snow_radar.id
}
