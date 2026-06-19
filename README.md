# Terraform Provider for OpenMediaVault

[![Terraform Registry](https://img.shields.io/badge/registry-sandy--rt%2Fomv-7B42BC?logo=terraform)](https://registry.terraform.io/providers/sandy-rt/omv/latest)
[![Release](https://img.shields.io/github/v/release/sandy-rt/terraform-provider-omv?sort=semver)](https://github.com/sandy-rt/terraform-provider-omv/releases)
[![CI](https://github.com/sandy-rt/terraform-provider-omv/actions/workflows/test.yml/badge.svg)](https://github.com/sandy-rt/terraform-provider-omv/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Terraform provider to manage [OpenMediaVault](https://www.openmediavault.org/)
shared folders and NFS exports declaratively, via OMV's JSON-RPC API.

📦 **Published on the Terraform Registry:**
[`sandy-rt/omv`](https://registry.terraform.io/providers/sandy-rt/omv/latest)

> Tested against OpenMediaVault 7 (Sandworm).

## Install

No manual build required — Terraform fetches it from the registry:

```hcl
terraform {
  required_providers {
    omv = {
      source  = "sandy-rt/omv"
      version = "~> 0.1"
    }
  }
}
```

Then run `terraform init`.

## Features

- `omv_shared_folder` — manage shared folders (create/read/update/delete/import)
- `omv_nfs_share` — manage NFS exports
- `omv_filesystems` (data source) — discover filesystems for `mntentref`
- **Destroy safety guard** — `allow_destroy = false` (default) makes the provider
  refuse to delete shares, even on `terraform destroy`
- Single `omv_apply` resource to deploy all staged changes once (OMV applies can
  take a long time and are rolled back if interrupted)

## How applies work

OMV separates *staging* config changes from *deploying* them. The
`omv_shared_folder` and `omv_nfs_share` resources only **stage** changes (fast).
Deployment happens when `Config.applyChanges` runs, which can take minutes to
~1 hour on low-powered hardware — and if interrupted, OMV reverts the staged
changes.

So this provider does **not** apply after every resource. Instead you declare a
single `omv_apply` resource that depends on your shares (via `triggers`) and runs
the deployment once, at the end of the run, waiting for it to finish.

## Usage

```hcl
terraform {
  required_providers {
    omv = {
      source  = "sandy-rt/omv"
      version = "~> 0.1"
    }
  }
}

provider "omv" {
  endpoint = "http://omv.example.com" # or OMV_ENDPOINT
  username = "admin"                  # or OMV_USERNAME
  password = var.omv_password         # or OMV_PASSWORD
}

data "omv_filesystems" "all" {}

resource "omv_shared_folder" "app" {
  name      = "app"
  comment   = "app data"
  mntentref = data.omv_filesystems.all.filesystems[0].uuid
}

resource "omv_nfs_share" "app" {
  sharedfolderref = omv_shared_folder.app.uuid
  client          = "*"
  options         = "rw"
  extraoptions    = "insecure,no_root_squash,subtree_check,sync"
  comment         = "app data"
}

# Deploys everything once, after the shares are staged. Re-runs whenever a
# referenced resource changes.
resource "omv_apply" "this" {
  triggers = {
    app_folder = sha1(jsonencode(omv_shared_folder.app))
    app_nfs    = sha1(jsonencode(omv_nfs_share.app))
  }
  timeout_minutes = 90
}
```

The `sharedfolderref` reference makes Terraform always create the shared folder
before the NFS export that uses it, and the `triggers` references make
`omv_apply` run after both.

## Provider configuration

| Argument | Env var | Required | Description |
|---|---|---|---|
| `endpoint` | `OMV_ENDPOINT` | yes | OMV base URL, e.g. `http://omv.example.com` |
| `username` | `OMV_USERNAME` | yes | OMV admin username |
| `password` | `OMV_PASSWORD` | yes | OMV admin password (sensitive) |
| `allow_destroy` | — | no | Default `false`. Must be `true` to allow deletes. |

Credentials are never written to Terraform state.

## Importing existing shares

```sh
terraform import omv_shared_folder.app <shared-folder-uuid>
terraform import omv_nfs_share.app     <nfs-export-uuid>
```

Import is read-only — it records an existing share in state without changing it.

## Documentation

Full resource/data-source docs are in [`docs/`](docs/) (rendered on the Terraform
Registry).

## Development

Requirements: [Go](https://go.dev/) >= 1.23 and
[Terraform](https://www.terraform.io/) >= 1.5.

```sh
# build + install into the local plugin directory so `terraform init` finds it
make install
```

This installs to
`~/.terraform.d/plugins/registry.terraform.io/sandy-rt/omv/<version>/<os>_<arch>/`.

Regenerate docs after schema changes:

```sh
tfplugindocs generate --provider-name omv
```

## Releasing

Pushing a `vX.Y.Z` tag triggers the GitHub Actions release workflow, which uses
[GoReleaser](https://goreleaser.com) to build cross-platform binaries and publish
a signed GitHub Release consumable by the Terraform Registry. Requires the repo
secrets `GPG_PRIVATE_KEY` and `PASSPHRASE`.

## License

[MIT](LICENSE)
