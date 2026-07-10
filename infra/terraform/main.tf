terraform {
  required_version = ">= 1.6.0"

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 5.0"
    }
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.45"
    }
  }

  backend "local" {}
}

provider "oci" {
  tenancy_ocid     = var.oci_tenancy_ocid
  user_ocid        = var.oci_user_ocid
  fingerprint      = var.oci_fingerprint
  private_key_path = var.oci_private_key_path
  region           = var.oci_region
}

provider "hcloud" {
  token = var.hcloud_token
}

module "oracle_sgp" {
  source = "./modules/oracle"

  compartment_id      = var.oci_compartment_id
  region              = var.oci_region
  availability_domain = var.oci_availability_domain
  instance_shape      = "VM.Standard.A1.Flex"
  instance_name       = "snow-radar-sgp"
  vcn_cidr            = "10.10.0.0/16"
  subnet_cidr         = "10.10.1.0/24"
  ssh_public_key      = var.ssh_public_key
  ocpu_count          = 2
  memory_gb           = 12
  boot_volume_gb      = 50
}

module "hetzner_fsn" {
  source = "./modules/hetzner"

  server_name   = "snow-radar-fsn"
  server_type   = "cx22"
  location      = "fsn1"
  ssh_public_key = var.ssh_public_key
}
