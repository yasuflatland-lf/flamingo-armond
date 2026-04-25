# Terraform — flamingo-armond infrastructure

Provisions the three production providers (Supabase, Render, Vercel) and wires
them together. Replaces the manual bring-up that previously lived in the
provider dashboards plus the legacy `render.yaml`.

`docs/deployment.md` is the operator-facing entry point — read that first for
the full bring-up flow, including manual prerequisites that cannot be
automated (PAT issuance, GitHub App installs, Google OAuth client creation).
This README is the local reference for the Terraform code itself.

## Layout

```
ops/terraform/
├── envs/prod/        # root module — call site that wires the three modules
└── modules/
    ├── supabase/     # supabase_project + supabase_settings + db_password
    ├── render/       # render_web_service + env vars
    └── vercel/       # vercel_project + env vars
```

Adding `envs/staging/` later is a directory copy with a separate
`mise.local.toml` — no module changes required.

## Initial setup

```bash
# 1. Install Terraform (1.14.9) via mise.
mise install
mise trust

# 2. Provide secrets.
cp ops/terraform/mise.local.example.toml ops/terraform/envs/prod/mise.local.toml
$EDITOR ops/terraform/envs/prod/mise.local.toml   # fill in 8 values

# 3. Apply.
cd ops/terraform/envs/prod
terraform init
terraform plan
terraform apply
```

`terraform apply` runs end-to-end (~15 min including provider provisioning).
The DAG resolves to a single straight line: Supabase project -> Render -> Vercel
-> Supabase settings update.

## Useful commands

```bash
terraform output                                  # all non-sensitive outputs
terraform output -raw supabase_db_password        # sensitive value
terraform output -raw production_url              # frontend URL
terraform plan -refresh-only                      # detect drift
terraform taint random_password.db                # rotate DB password
```

## Provider versions

Pinned in `versions.tf` at every level:

| Provider | Constraint | Source |
|---|---|---|
| `supabase` | `~> 1.5` | `supabase/supabase` |
| `render` | `~> 1.0` | `render-oss/render` |
| `vercel` | `~> 2.0` | `vercel/vercel` |
| `random` | `~> 3.6` | `hashicorp/random` |

Terraform CLI is pinned at `1.14.9` via `mise.toml`.

## Out of scope

These are intentionally not Terraformed:

- Database schema migrations — `golang-migrate` runs at backend boot.
- Supabase Storage policies / RLS — feature not yet used.
- Custom domains — defaults (`*.vercel.app`, `*.onrender.com`) are sufficient.
- CI integration — `terraform apply` runs locally only for now.
- Auth users — created by Google OAuth at runtime.
- Manual prerequisites — see `docs/deployment.md`.

## Failure modes

| Situation | Recovery |
|---|---|
| Apply fails midway | Re-run `terraform apply`. All three providers' resources are idempotent. |
| `mise.local.toml` lost | Rotate at each provider's dashboard, refill, re-apply. State is unaffected. |
| State file lost / corrupted | `terraform import` each resource; refs come from the provider dashboards. |
| Want to start over | `terraform destroy` (lifts the `prevent_destroy` guard manually first). DB data is unrecoverable. |

## Why split `supabase_project` and `supabase_settings`?

The Vercel domain is needed for Supabase Auth's Site URL and redirect allow
list. Keeping settings separate from the project lets Terraform's DAG resolve
the loopback as a normal forward edge: `supabase_project -> vercel -> supabase_settings`.
A single-resource design would create a cycle and require two-pass apply.
