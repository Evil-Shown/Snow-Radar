# Snow Radar — GitHub, Domain & Cloud Setup

Complete this checklist **before** running Terraform.

---

## 1. GitHub Organization

### Create Org
1. Go to https://github.com/organizations/new
2. Organization name: `snow-radar`
3. Plan: Free (unlimited private repos)
4. Owner: Your personal account

### Create Repositories
Create three **private** repositories:

| Repo | Description | Topics |
|------|-------------|--------|
| `snowradar-infra` | Terraform, Ansible, Docker Compose, ADRs, docs | `terraform`, `ansible`, `infrastructure`, `vpn` |
| `snowradar-api` | Go control plane, migrations, Dockerfile | `go`, `gin`, `wireguard`, `api` |
| `snowradar-client` | Flutter apps (mobile + desktop), build scripts | `flutter`, `dart`, `vpn`, `wireguard` |

**Branch Protection (all repos):**
```
Settings → Branches → Add rule for 'main'
☑ Require a pull request before merging
  ☑ Require approvals: 1
☑ Require status checks to pass before merging
  ☑ Require branches to be up to date
☑ Require conversation resolution before merging
☑ Include administrators
```

### Machine User for CI/CD
1. Create GitHub account: `snowradar-bot` (use org email)
2. Invite to org with **Write** access to packages, **Read** to repos
3. Generate PAT (Classic): `Settings → Developer settings → Personal access tokens → Classic`
   - Scopes: `repo`, `write:packages`, `read:packages`, `workflow`
4. Save token as `SNOWRADAR_BOT_PAT` in 1Password / Bitwarden

### Repository Secrets (Org-level)
```
Settings → Secrets and variables → Actions → Organization secrets
```
| Secret | Value | Used By |
|--------|-------|---------|
| `SNOWRADAR_BOT_PAT` | `ghp_xxx` | All workflows |
| `OCI_TENANCY_OCID` | `ocid1.tenancy...` | Terraform |
| `OCI_USER_OCID` | `ocid1.user...` | Terraform |
| `OCI_FINGERPRINT` | `xx:xx:xx...` | Terraform |
| `OCI_PRIVATE_KEY` | (multi-line PEM) | Terraform |
| `OCI_REGION` | `ap-singapore-1` | Terraform |
| `OCI_COMPARTMENT_OCID` | `ocid1.compartment...` | Terraform |
| `HCLOD_TOKEN` | `your-hetzner-api-token` | Terraform |
| `SSH_PUBLIC_KEY` | `ssh-ed25519 AAAA...` | Terraform, Ansible |
| `DOCKER_REGISTRY` | `ghcr.io` | Docker build |
| `GO_VERSION` | `1.22` | Go workflows |
| `FLUTTER_VERSION` | `3.22` | Flutter workflows |

### Repository Variables (Org-level)
```
Settings → Secrets and variables → Actions → Variables
```
| Variable | Value |
|----------|-------|
| `ORG_NAME` | `snow-radar` |
| `API_IMAGE` | `ghcr.io/snow-radar/snowradar-api` |
| `CLIENT_ANDROID_PACKAGE` | `com.snowradar.client` |

---

## 2. Domain Registration

### Register
- Preferred: `snowradar.app` (Google Registry, HSTS preloaded)
- Alternatives: `snowradar.io`, `snowradar.network`, `snowradar.vpn`
- Registrar: Cloudflare, Porkbun, Namecheap (avoid GoDaddy)

### DNS Configuration
After Terraform gives you IPs, create records:

| Type | Name | Value | TTL | Proxy |
|------|------|-------|-----|-------|
| A | `@` | `<ORACLE_IP>` | 300 | DNS only |
| A | `eu` | `<HETZNER_IP>` | 300 | DNS only |
| A | `api` | `<ORACLE_IP>` | 300 | DNS only |
| CNAME | `www` | `snowradar.app` | 300 | DNS only |
| TXT | `@` | `v=spf1 include:_spf.google.com ~all` | 3600 | - |
| TXT | `_dmarc` | `v=DMARC1; p=none; rua=mailto:dmarc@snowradar.app` | 3600 | - |

**CAA Records (for Let's Encrypt):**
| Type | Name | Value |
|------|------|-------|
| CAA | `@` | `0 issue "letsencrypt.org"` |
| CAA | `@` | `0 issuewild "letsencrypt.org"` |
| CAA | `@` | `0 iodef "mailto:security@snowradar.app"` |

### DNSSEC
Enable at registrar (one click on Cloudflare/Porkbun).

---

## 3. Oracle Cloud (Always Free)

### Sign Up
1. https://cloud.oracle.com/free
2. Verify phone + credit card (no charge for Always Free)
3. Select Home Region: **Singapore (AP-SINGAPORE-1)**

### Create API Key
1. Console → Profile → User Settings → API Keys → **Add API Key**
2. Generate RSA key pair:
   ```bash
   mkdir -p ~/.oci
   openssl genrsa -out ~/.oci/oci_api_key.pem 2048
   chmod 600 ~/.oci/oci_api_key.pem
   openssl rsa -pubout -in ~/.oci/oci_api_key.pem -out ~/.oci/oci_api_key_public.pem
   ```
3. Paste public key content in Oracle Console
4. Note down:
   - **Fingerprint** (e.g., `aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99`)
   - **Tenancy OCID** (root compartment)
   - **User OCID** (your user)
   - **Compartment OCID** (use root or create dedicated)
   - **Availability Domain** (e.g., `AD-1`, `AD-2`, `AD-3`)

### Verify CLI (Optional)
```bash
pip install oci-cli
oci setup config  # Follow prompts
oci iam availability-domain list --compartment-id <TENANCY_OCID>
```

### Limits Check
Always Free includes:
- 4 AMD CPUs or 4 ARM OCpus (VM.Standard.A1.Flex)
- 24 GB RAM (ARM)
- 200 GB Block Volume
- 10 TB/month egress

---

## 4. Hetzner Cloud

### Sign Up
1. https://console.hetzner.cloud
2. Create Project: `snowradar`
3. Add payment method (€4.51/mo for CX22)

### API Token
1. Project → Security → API Tokens → **Generate Token**
2. Name: `terraform`, Permissions: **Read & Write**
3. Copy token (shown once!)

### SSH Key
1. Project → Security → SSH Keys → **Add SSH Key**
2. Name: `snowradar-infra`, paste `~/.ssh/snowradar.pub` content

### Verify
```bash
curl -H "Authorization: Bearer $HCLOD_TOKEN" \
  https://api.hetzner.cloud/v1/servers
```

---

## 5. Local Machine Setup

### SSH Key for Infrastructure
```bash
# If not exists
ssh-keygen -t ed25519 -C "snowradar-infra" -f ~/.ssh/snowradar
# Public key: ~/.ssh/snowradar.pub → add to GitHub secrets, Oracle, Hetzner
```

### Tool Versions
```bash
# Terraform
terraform version  # >= 1.6

# Go
go version  # >= 1.22

# Flutter
flutter --version  # >= 3.22

# Docker
docker --version  # >= 24

# GitHub CLI
gh auth login  # Authenticate to org
```

### Clone Repos
```bash
mkdir -p ~/code/snowradar
cd ~/code/snowradar
gh repo clone snow-radar/snowradar-infra
gh repo clone snow-radar/snowradar-api
gh repo clone snow-radar/snowradar-client
```

---

## 6. Terraform Configuration

### Configure Variables
```bash
cd snowradar-infra/infra/terraform
cp terraform.tfvars.example terraform.tfvars
# Edit with your values (see terraform.tfvars.example)
```

### Required `terraform.tfvars` Fields
```hcl
oci_tenancy_ocid       = "ocid1.tenancy.oc1..xxxx"
oci_user_ocid          = "ocid1.user.oc1..xxxx"
oci_fingerprint        = "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99"
oci_private_key_path   = "~/.oci/oci_api_key.pem"
oci_region             = "ap-singapore-1"
oci_compartment_id     = "ocid1.compartment.oc1..xxxx"
oci_availability_domain = "AD-1"  # or empty for auto

hcloud_token = "your-hetzner-api-token"

ssh_public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... snowradar-infra"
```

### Deploy
```bash
terraform init
terraform fmt
terraform validate
terraform plan
terraform apply
```

### Outputs to Save
```bash
terraform output -json > ../tf-outputs.json
# Contains: oracle_instance_public_ip, hetzner_server_public_ip
```

---

## 7. Post-Deploy Verification

### SSH Access
```bash
ssh -i ~/.ssh/snowradar ubuntu@<ORACLE_IP>
ssh -i ~/.ssh/snowradar root@<HETZNER_IP>  # Hetzner uses root initially
```

### DNS Update
Update your domain A records with the two IPs from Terraform output.

### Test Connectivity
```bash
# From local machine
curl -I https://snowradar.app
curl -I https://eu.snowradar.app
curl -I https://api.snowradar.app
```

---

## 8. CI/CD Bootstrap

### Add Workflow Files
Each repo needs:
- `.github/workflows/ci.yml` (lint, test)
- `.github/workflows/cd.yml` (build, push, deploy)

See `docs/operations/GITHUB_WORKFLOWS.md` for templates.

### Environments
Create GitHub Environments: `staging`, `production`
- `production` → Require approval
- Protection rules: Required reviewers, deployment branches

---

## 9. Cost Monitoring

### Oracle
- Always Free: $0 (monitor for "Always Free resources will be reclaimed" emails)
- Set budget alert at $1/mo in Oracle Console → Budgets

### Hetzner
- Project → Billing → Set budget alert at €10
- Enable monthly invoice email

### GitHub
- Free org: 2,000 CI minutes/mo private repos
- Monitor: Settings → Billing

---

## 10. Security Baseline

| Item | Status | Notes |
|------|--------|-------|
| Org 2FA enforced | ☐ | Settings → Authentication security |
| Branch protection | ☐ | All 3 repos |
| Dependabot alerts | ☐ | Enable on all repos |
| Code scanning (CodeQL) | ☐ | Enable on api + client |
| Secret scanning | ☐ | Enable on all repos |
| Signed commits | ☐ | Require on main |
| PAT rotation schedule | ☐ | Quarterly calendar reminder |

---

## Checklist Summary

- [ ] GitHub org `snow-radar` created
- [ ] 3 private repos created with branch protection
- [ ] Machine user `snowradar-bot` invited, PAT stored
- [ ] Org secrets + variables configured
- [ ] Domain registered, DNSSEC enabled
- [ ] Oracle Cloud account, API key generated, limits noted
- [ ] Hetzner Cloud account, API token, SSH key added
- [ ] Local tools installed (TF, Go, Flutter, Docker, gh)
- [ ] Repos cloned locally
- [ ] `terraform.tfvars` filled, `terraform apply` succeeds
- [ ] DNS records updated with Terraform IPs
- [ ] SSH access verified to both servers
- [ ] CI/CD workflows added to all repos
- [ ] Cost alerts configured
- [ ] Security baseline complete

---

**Next Step:** Proceed to [Phase 1 Server Hardening](SETUP.md#phase-1-bare-metal-vpn--observability-week-2)