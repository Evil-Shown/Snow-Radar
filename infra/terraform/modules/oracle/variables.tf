variable "compartment_id" {
  type = string
}

variable "region" {
  type = string
}

variable "availability_domain" {
  type    = string
  default = ""
}

variable "instance_shape" {
  type    = string
  default = "VM.Standard.A1.Flex"
}

variable "instance_name" {
  type = string
}

variable "vcn_cidr" {
  type    = string
  default = "10.10.0.0/16"
}

variable "subnet_cidr" {
  type    = string
  default = "10.10.1.0/24"
}

variable "ssh_public_key" {
  type = string
}

variable "ocpu_count" {
  type    = number
  default = 2
}

variable "memory_gb" {
  type    = number
  default = 12
}

variable "boot_volume_gb" {
  type    = number
  default = 50
}
