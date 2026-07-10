variable "server_name" {
  type = string
}

variable "server_type" {
  type    = string
  default = "cx22"
}

variable "location" {
  type    = string
  default = "fsn1"
}

variable "ssh_public_key" {
  type = string
}
