# ADR-001: Multi-Provider Infrastructure with Oracle Cloud and Hetzner

## Status
Accepted

## Context
Snow Radar requires a privacy-first VPN platform with geographically distributed exit nodes. We need infrastructure in APAC (Singapore) and Europe (Frankfurt region) to serve users in both regions with low-latency connections.

## Decision
- **Oracle Cloud (AP-Singapore-1):** ARM-based `VM.Standard.A1.Flex` instance (2 OCPUs, 12 GB RAM, 50 GB boot) running Ubuntu 22.04. Chosen for its generous Always Free tier and ARM architecture.
- **Hetzner Cloud (fsn1 - Falkenstein, Germany):** CX22 instance (2 vCPUs, 4 GB RAM, 40 GB SSD) running Ubuntu 22.04. Note: Hetzner does not operate a Frankfurt datacenter; Falkenstein (fsn1) is the closest available location in Germany.
- **Networking:** Both instances expose SSH (22/tcp), HTTP (80/tcp), HTTPS (443/tcp), and WireGuard (51820/udp).
- **IaC Tool:** Terraform with modular structure for multi-provider management.

## Consequences
- Positive: Two-region coverage across APAC and EU; cost-effective with Oracle Always Free and Hetzner's competitive pricing.
- Positive: Modular Terraform structure allows adding more providers/regions without modifying core config.
- Negative: Managing two cloud providers increases operational complexity (auth, billing, monitoring).
- Negative: Hetzner's closest location to Frankfurt is Falkenstein (~300 km), which may slightly affect latency expectations.

## References
- Oracle Cloud ARM shapes: https://docs.oracle.com/en-us/iaas/Content/Compute/References/computeshapes.htm
- Hetzner Cloud locations: https://docs.hetzner.com/cloud/general/locations
