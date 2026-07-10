# ADR-003: Multi-Cloud Infrastructure with Oracle Cloud (Singapore) + Hetzner (Falkenstein)

## Status
Accepted

## Context
Snow Radar needs two geographically distributed exit nodes at launch:
- **APAC node**: Serve Southeast Asia, Australia, Pacific users
- **EU node**: Serve Europe, Middle East, Africa users

Constraints:
- **Budget**: Pre-revenue, founder-funded; must minimize fixed costs
- **Architecture**: ARM preferred for efficiency; x86 acceptable for compatibility
- **Censorship relevance**: Singapore has open internet; Germany has strong privacy laws
- **Operational simplicity**: Prefer managed VMs over bare metal or Kubernetes at this stage

## Decision
| Region | Provider | Instance | Specs | Monthly Cost |
|--------|----------|----------|-------|--------------|
| **APAC (Singapore)** | Oracle Cloud | VM.Standard.A1.Flex (ARM) | 2 OCPU, 12 GB RAM, 50 GB boot, 10 TB transfer | **$0** (Always Free) |
| **EU (Germany)** | Hetzner Cloud | CX22 (x86) | 2 vCPU, 4 GB RAM, 40 GB SSD, 20 TB transfer | **€4.51** (~$4.90) |

**Notes:**
- Oracle Always Free: 4 ARM OCPUs + 24 GB RAM total across tenancy; we use 2 OCPUs for VPN
- Hetzner: Falkenstein (FSN1) is the closest location to Frankfurt; Hetzner has no Frankfurt datacenter
- Both run Ubuntu 22.04 LTS
- Terraform manages both providers in single state

## Consequences

### Positive
- **Cost**: ~$5/mo for two exit nodes (vs $20-40 on AWS/GCP/Azure)
- **ARM experience**: Valuable for future ARM-only deployments (Graviton, Ampere)
- **Dere)
- **Diversity**: Two providers = no single point of cloud-provider failure
- **Privacy jurisdictions**: Singapore (PDPA) + Germany (GDPR) - both strong
- **Transfer allowances**: 10 TB + 20 TB = 30 TB/mo included (plenty for MVP)

### Negative
- **Two providers**: Double the credential management, billing, support channels
- **Hetzner location**: Falkenstein (~300 km from Frankfurt) adds ~3-5 ms latency vs true Frankfurt
- **Oracle Always Free limits**: Cannot scale beyond 4 OCPUs without paying; account termination risk if idle
- **ARM vs x86 parity**: Must test both architectures; AmneziaWG Go binary works on both but kernel WireGuard differs
- **No private networking**: Cross-cloud VPC peering not available; inter-node traffic goes over public internet

### Neutral / Risks
- **Oracle account suspension**: Reports of Always Free accounts terminated without notice; mitigate with backup provider research
- **Hetzner abuse complaints**: Strict anti-abuse; ensure port 25 blocked, monitor for spam/torrent complaints
- **IPv6**: Both support but Oracle IPv6 is /64 per VNIC; Hetzner gives /64 per server

## Alternatives Considered

| Alternative | Pros | Cons | Why Not Chosen |
|-------------|------|------|----------------|
| **AWS (t4g.micro + t3.micro)** | Global, reliable | Free tier expires 12 mo; then ~$15/mo each; complex billing | Cost after free tier |
| **GCP (e2-micro + e2-micro)** | Always free f1-micro (x86) | f1-micro too weak (0.25 vCPU); e2-micro not free in all regions | Performance |
| **Azure (B1ls + B1ls)** | Free tier 12 mo | ARM not free; complex networking | Cost + complexity |
| **DigitalOcean (2x $4)** | Simple, predictable | $8/mo base; no ARM option in SG | Cost |
| **Vultr (2x $6)** | Global, ARM in SG | $12/mo | Cost |
| **Self-hosted (2x mini PCs)** | Zero monthly | Power, bandwidth, hardware failure, static IP issues | Operational burden |
| **Single provider multi-region** | Simpler ops | No Always Free ARM in EU on Oracle; Hetzner no APAC | Constraints |

## Implementation Details

### Terraform Structure
```
infra/terraform/
├── main.tf                 # Providers, module calls
├── variables.tf            # All inputs
├── outputs.tf              # Public IPs, instance IDs
├── terraform.tfvars.example
└── modules/
    ├── oracle/
    │   ├── main.tf         # VCN, subnet, IGW, sec list, instance
    │   ├── variables.tf
    │   └── outputs.tf
    └── hetzner/
        ├── main.tf         # Firewall, SSH key, server
        ├── variables.tf
        └── outputs.tf
```

### Oracle Module Key Resources
- `oci_core_vcn` (10.10.0.0/16)
- `oci_core_subnet` (10.10.1.0/24)
- `oci_core_internet_gateway`
- `oci_core_route_table` (default route to IGW)
- `oci_core_security_list` (ingress: 22, 80, 443, 51820/udp)
- `oci_core_instance` (VM.Standard.A1.Flex, Ubuntu 22.04, cloud-init)

### Hetzner Module Key Resources
- `hcloud_firewall` (rules for 22, 80, 443, 51820/udp)
- `hcloud_ssh_key` (from var)
- `hcloud_server` (cx22, fsn1, Ubuntu 22.04, user_data=cloud-init)

### Cloud-Init (Both)
- Create `snowradar` sudo user with SSH key
- Disable root SSH
- Enable IP forwarding (sysctl)
- Install wireguard, ufw, docker
- Configure UFW allow rules

## References
- [Oracle Always Free](https://www.oracle.com/cloud/free/)
- [Hetzner Cloud Pricing](https://www.hetzner.com/cloud)
- [Hetzner Locations](https://docs.hetzner.com/cloud/general/locations)
- [OCI ARM Shapes](https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm#arm)

## Metadata
- **Date**: 2025-01-15
- **Author**: Snow Radar Founder
- **Reviewers**: N/A
- **Tags**: infrastructure, cloud, oracle, hetzner, multi-cloud, cost-optimization