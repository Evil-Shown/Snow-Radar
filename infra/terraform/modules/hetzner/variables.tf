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

variable "wg_port" {
  description = "UDP port for the standard WireGuard tunnel (wg0). Must match infra/configs/subnets.env (ADR-004)."
  type        = number
  default     = 51820
}

variable "awg_port" {
  description = "UDP port for the AmneziaWG stealth tunnel (awg0). Must match infra/configs/subnets.env (ADR-004)."
  type        = number
  default     = 51821
}
