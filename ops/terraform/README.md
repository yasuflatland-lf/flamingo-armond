# Terraform — flamingo-armond infrastructure

Provisions the three production providers (Supabase, Render, Vercel) and wires
them together. Terraform is the single source of truth for the production
infrastructure shape; provider dashboards are read-only follow-along surfaces.

`docs/deployment.md` is the operator-facing entry point — read that first for
the full bring-up flow, including manual prerequisites that cannot be
automated (PAT issuance, GitHub App installs, Google OAuth client creation).
This README is the local reference for the Terraform code itself.

## Layout

```
ops/terraform/
├── envs/prod/
│   ├── initial/      # creates supabase project + render service + vercel project
│   └── settings/     # supabase auth config (site_url, redirect URLs, OAuth)
└── modules/
    ├── supabase/     # supabase_project + db_password + JWKS/pooler outputs
    ├── render/       # render_web_service + env vars
    └── vercel/       # vercel_project + env vars
```

The two stacks under `envs/prod/` are applied in order. The `settings` stack
reads the `initial` stack's local state via `terraform_remote_state` to wire
the Vercel hostname into Supabase Auth. Splitting them keeps the DB password
state out of the resource that operators iterate on most often.

Adding `envs/staging/` later is a directory copy with a separate
`mise.local.toml` — no module changes required.

> **WARNING — local state.** Both stacks use Terraform's local backend. State
> files (`envs/prod/initial/terraform.tfstate`, `envs/prod/settings/terraform.tfstate`)
> contain the DB password and other sensitive values in plaintext on whoever
> ran `apply`. Do not run `terraform apply` from a second machine without
> migrating to a remote backend first; a fresh local state will attempt to
> recreate every resource. Remote backend selection is tracked separately.

## Initial setup

```bash
# 1. Install Terraform (1.14.9) via mise.
mise install
mise trust

# 2. Provide secrets (single shared file, inherited by both stacks).
cp ops/terraform/mise.local.example.toml ops/terraform/envs/prod/mise.local.toml
$EDITOR ops/terraform/envs/prod/mise.local.toml   # fill 7 secrets

# 3. Apply initial — creates supabase project, render service, vercel project.
cd ops/terraform/envs/prod/initial
terraform init
terraform plan
terraform apply

# 4. Apply settings — wires Vercel hostname into Supabase Auth + Google OAuth.
cd ../settings
terraform init
terraform plan
terraform apply
```

`terraform apply` for `initial` runs end-to-end (~15 min including provider
provisioning). `settings` is a single resource and applies in seconds.

## Useful commands

Run from the relevant stack directory.

```bash
# Initial stack (envs/prod/initial)
terraform output                                  # all non-sensitive outputs
terraform output -raw supabase_db_password        # sensitive value
terraform output -raw production_url              # frontend URL
terraform output -raw render_service_id           # for the first manual deploy
terraform plan -refresh-only                      # detect drift
terraform apply -replace=module.supabase.random_password.db   # rotate DB password
```

```bash
# Settings stack (envs/prod/settings)
terraform output                                  # site_url + supabase_project_ref
terraform apply                                   # re-apply auth config after dashboard drift
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
  When a custom domain is added, set `TF_VAR_vercel_production_domain_override`
  so the settings stack wires Auth Site URL to the canonical hostname.
- CI integration — `terraform apply` runs locally only for now. CI does run
  `terraform fmt -check` and `terraform validate` on every PR (see
  `.github/workflows/terraform.yml`).
- Auth users — created by Google OAuth at runtime.
- Manual prerequisites — see `docs/deployment.md`.

## Failure modes

| Situation | Recovery |
|---|---|
| `initial apply` fails midway | Re-run `terraform apply`. All three providers' resources are idempotent. |
| `settings apply` fails | Re-run from `envs/prod/settings/`. The DAG only revisits `supabase_settings`. |
| `mise.local.toml` lost | Rotate at each provider's dashboard, refill, re-apply. State is unaffected. |
| State file lost / corrupted | `terraform import` each resource; refs come from the provider dashboards. |
| Want to start over | Manually remove `prevent_destroy` blocks first, then `terraform destroy` from `settings` and `initial` in that order. DB data is unrecoverable. |

## Why split `initial` and `settings`?

Two reasons compound:

1. **Loopback dependency.** Supabase Auth's Site URL needs the Vercel
   hostname, but Vercel is created after Supabase. Keeping
   `supabase_settings` in a downstream stack lets Terraform's DAG resolve the
   loopback as a normal forward edge: `supabase_project -> vercel -> supabase_settings`.
2. **Iteration cadence.** Auth changes (redirect URLs, OAuth providers, custom
   domain) reapply often. The `initial` stack stays untouched for those
   changes, keeping DB password state isolated.
