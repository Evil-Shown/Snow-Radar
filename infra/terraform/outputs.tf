output "oracle_instance_public_ip" {
  description = "Public IP of the Oracle Cloud ARM instance"
  value       = module.oracle_sgp.instance_public_ip
}

output "oracle_instance_id" {
  description = "OCID of the Oracle Cloud ARM instance"
  value       = module.oracle_sgp.instance_id
}

output "hetzner_server_public_ip" {
  description = "Public IP of the Hetzner CX22 server"
  value       = module.hetzner_fsn.server_public_ip
}

output "hetzner_server_id" {
  description = "ID of the Hetzner server"
  value       = module.hetzner_fsn.server_id
}
