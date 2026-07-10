# Snow Radar — GitHub, Domain & Cloud Setup Checklist

This guide covers the one-time setup before Terraform can run.

---

## 1. GitHub Organization & Repositories

### Create Organization
1. Go to https://github.com/organizations/new
2. Organization name: `snow-radar`
3. Plan: Free (private repos included)
4. Invite your personal account as Owner

### Create Repositories
Create three **private** repositories:

| Repository | Description | Default Branch |
|------------|-------------|----------------|
| `snowradar-infra` | Terraform, Ansible, Docker Compose, ADRs, docs | `main` |
| `snowradar-api` | Go control plane, migrations, Dockerfile | `main` |
| `snowradar-client` | Flutter apps (mobile, desktop), build scripts | `main` |

**Branch Protection (all repos):**
- Settings → Branches → Add rule for `main`
- Require PR review (1)
- Require status checks to pass
- Require conversation resolution
- Include administrators

### Machine User for CI/CD
1. Create GitHub account: `snowradar-bot`
2. Invite to org with **Write** access to packages, **Read** to repos
3. Generate PAT (Classic) with scopes: `repo`, `write:packages`, `read:packages`
4. Store as `GH_BOT_TOKEN` in repo secrets

### Repository Secrets (per repo)
**snowradar-infra:**
- `TF_VAR_oci_tenancy_ocid`
- `TF_VAR_oci_user_ocid`
- `TF_VAR_oci_fingerprint`
- `TF_VAR_oci_private_key` (base64 encoded)
- `TF_VAR_hcloud_token`

**snowradar-api:**
- `DATABASE_URL` (production)
- `REDIS_URL` (production)
- `JWT_SECRET` (64-char random)
- `WG_INTERFACE` = `wg0`

**snowradar-client:**
- `API_BASE_URL` = `https://api.snowradar.app`
- `APP_STORE_ID` (for iOS)

---

## 2. Domain Registration & DNS

### Recommended Registrars
- **Cloudflare** (no markup, free WHOIS, API for automation)
- **Porkbun** (cheap, good API)
- **Namecheap** (reliable)

### Domain Choice
- `snowradar.app` — .app forces HTTPS (HSTS preload), good for VPN brand
- `snowradar.io` — classic tech TLD
- `snowradar.network` — descriptive

### DNS Records (after Terraform)
| Type | Name | Value | TTL | Proxy |
|------|------|-------|-----|-------|
| A | @ | `<oracle-ip>` | 300 | DNS only |
| A | eu | `<hetzner-ip>` | 300 | DNS only |
| A | api | `<oracle-ip>` | 300 | DNS only |
| CNAME | www | @ | 300 | DNS only |
| TXT | @ | `v=spf1 include:_spf.google.com ~all` | 3600 | - |
| CAA | @ | `0 issue "letsencrypt.org"` | 3600 | - |

**Do NOT** enable Cloudflare proxy (orange cloud) for VPN endpoints — WireGuard is UDP and won't work through Cloudflare's HTTP/HTTPS proxy.

### DNSSEC
Enable at registrar. Cloudflare does this automatically.

---

## 3. Oracle Cloud (Always Free)

### Sign Up
1. https://cloud.oracle.com → Start Free
2. Verify phone + credit card (no charge for Always Free)
3. Select home region: **Singapore (ap-singapore-1)**

### Create API Signing Key
```bash
# On your local machine
mkdir -p ~/.oci
openssl genrsa -out ~/.oci/oci_api_key.pem 2048
chmod 600 ~/.oci/oci_api_key.pem
openssl rsa -pubout -in ~/.oci/oci_api_key.pem -out ~/.oci/oci_api_key_public.pem
```

In OCI Console:
1. Profile → User Settings → API Keys → Add API Key
2. Paste contents of `~/.oci/oci_api_key_public.pem`
3. Note: **Fingerprint**, **Tenancy OCID**, **User OCID**

### Compartment
- Use root compartment or create dedicated: `snowradar`
- Note **Compartment OCID**

### Availability Domain
- Identity → Availability Domains
- Note name (e.g., `AD-1`)

### SSH Key for Instances
```bash
# Use same key as Terraform
cat ~/.ssh/snowradar.pub
```
Paste into OCI Console → Compute → Key Pairs (optional, Terraform injects via cloud-init)

### Limits Check
- Governance → Limits → `VM.Standard.A1.Flex` → Ensure 4 OCPUs available in Singapore

---

## 4. Hetzner Cloud

### Sign Up
1. https://console.hetzner.cloud → Register
2. Verify email + phone
3. Add payment method (€4.51/mo for CX22)

### Create Project
- Projects → New Project → `snowradar`

### API Token
- Security → API Tokens → Generate
- Name: `terraform`
- Permissions: **Read & Write**
- **Copy immediately** — shown once!

### SSH Key
- Security → SSH Keys → Add
- Name: `snowradar-terraform`
- Paste `~/.ssh/snowradar.pub`

### Location
- **Falkenstein (fsn1)** — only German location with CX22
- Note: Nuremberg (nbg1) has CX22 but fsn1 is default

---

## 5. Local Development Environment

### Required Tools
```bash
# macOS
brew install git terraform go docker docker-compose fluxctl kubectl helm

# Windows (PowerShell as Admin)
winget install Git.Git HashiCorp.Terraform GoLang.Go Docker.DockerDesktop

# Linux (Ubuntu/Debian)
sudo apt update && sudo apt install -y git terraform golang-go docker.io docker-compose-plugin
```

### Verify
```bash
git --version
terraform version
go version
docker --version
docker compose version
```

### SSH Agent
```bash
eval $(ssh-agent -s)
ssh-add ~/.ssh/snowradar
ssh-add -l  # Should show your key
```

### GPG for Commits (Optional)
```bash
gpg --full-generate-key
git config --global user.signingkey <KEY_ID>
git config --global commit.gpgsign true
```

---

## 6. Clone & Bootstrap

```bash
mkdir -p ~/code/snowradar
cd ~/code/snowradar

# Clone all three
gh repo clone snow-radar/snowradar-infra
gh repo clone snow-radar/snowradar-api
gh repo clone snow-radar/snowradar-client

# Bootstrap infra
cd snowradar-infra/infra/terraform
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
terraform init
terraform fmt
terraform validate
```

---

## 7. First Deploy

```bash
terraform plan -out=tfplan
terraform apply tfplan
```

**Save outputs:**
```bash
terraform output -json > ../tf-outputs.json
```

Update DNS with the two public IPs.

---

## 8. Verify Server Access

```bash
# Oracle (Singapore)
ssh -i ~/.ssh/snowradar ubuntu@<oracle-ip>

# Hetzner (Falkenstein)
ssh -i ~/.ssh/snowradar root@<hetzner-ip>
# Then create admin user per Phase 1 guide
```

---

## 9. Post-Deploy Checklist

- [ ] GitHub org created with 3 repos
- [ ] Branch protection enabled
- [ ] Domain registered, DNSSEC on
- [ ] Oracle API key + compartment OCID saved
- [ ] Hetzner API token + SSH key saved
- [ ] `terraform.tfvars` filled (never commit!)
- [ ] `terraform apply` succeeds
- [ ] DNS A records updated
- [ ] SSH works to both servers
- [ ] `ufw status` shows correct rules
- [ ] `wg show` shows interface up
- [ ] Node Exporter metrics at `:9100/metrics`

---

## 10. Useful Commands

```bash
# Re-deploy after changes
cd ~/code/snowradar/snowradar-infra/infra/terraform
terraform plan && terraform apply

# Destroy everything (careful!)
terraform destroy

# View outputs
terraform output

# SSH with agent forwarding (for Ansible later)
ssh -A -i ~/.ssh/snowradar ubuntu@<ip>
```

---

## Security Notes

- **Never commit** `terraform.tfvars`, `.tfstate`, or SSH private keys
- Rotate API tokens quarterly
- Use GitHub Environments for production secrets
- Enable 2FA on all accounts (GitHub, Oracle, Hetzner, registrar)
- Monitor billing alerts on both clouds

---

**Next**: Proceed to Phase 1 server hardening (SETUP.md → Phase 1 section)