data "hcloud_image" "ubuntu_2204" {
  name = "ubuntu-22.04"
}

resource "hcloud_ssh_key" "snow_radar" {
  name       = "${var.server_name}-key"
  public_key = var.ssh_public_key
}

resource "hcloud_firewall" "snow_radar" {
  name = "${var.server_name}-fw"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = ["0.0.0.0/0", "::/0"]
  }

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

  rule {
    direction  = "in"
    protocol   = "udp"
    port       = "51820"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "snow_radar" {
  name        = var.server_name
  server_type = var.server_type
  location    = var.location
  image       = data.hcloud_image.ubuntu_2204.id
  ssh_keys    = [hcloud_ssh_key.snow_radar.id]
  firewall_ids = [hcloud_firewall.snow_radar.id]

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }
}
