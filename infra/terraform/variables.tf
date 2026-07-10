variable "oci_tenancy_ocid" {
  description = "OCID of the OCI tenancy"
  type        = string
  sensitive   = true
}

variable "oci_user_ocid" {
  description = "OCID of the OCI user"
  type        = string
  sensitive   = true
}

variable "oci_fingerprint" {
  description = "Fingerprint of the OCI API signing key"
  type        = string
}

variable "oci_private_key_path" {
  description = "Path to the OCI API private key"
  type        = string
}

variable "oci_region" {
  description = "OCI region identifier"
  type        = string
  default     = "ap-singapore-1"
}

variable "oci_compartment_id" {
  description = "OCID of the OCI compartment for resources"
  type        = string
}

variable "oci_availability_domain" {
  description = "Availability domain for the Oracle instance"
  type        = string
  default     = ""
}

variable "hcloud_token" {
  description = "Hetzner Cloud API token"
  type        = string
  sensitive   = true
}

variable "ssh_public_key" {
  description = "SSH public key for instance access"
  type        = string
}
