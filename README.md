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
- **Automatic deploy** — changes are deployed (`Config.applyChanges`) and waited
  on automatically as part of `terraform apply` *and* `terraform destroy`. No
  extra resource or wrapper needed.
- **Destroy safety guard** — `allow_destroy = false` (default) makes the provider
  refuse to delete shares, even on `terraform destroy`

## How applies work

OMV separates *staging* config changes from *deploying* them. After staging a
change, this provider calls `Config.applyChanges` and **waits for it to finish**
— so `terraform apply`/`destroy` leave the NAS fully deployed, with no pending
"apply changes" banner.

Notes:
- OMV applies can take minutes (longer on low-powered hardware). Terraform shows
  `Still creating... [Ns elapsed]` while it waits. Tune the max wait with
  `apply_timeout_minutes` (default 120).
- Deploys are **serialized** inside the provider, so parallel resource changes
  don't trigger overlapping applies (which would cancel each other's restarts).
- The session is **re-established automatically** if it expires mid-apply.
- Each changed resource triggers a deploy, so a run that changes N resources
  performs N (sequential) deploys.

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
```

That's it — `terraform apply` creates and **deploys** both. The `sharedfolderref`
reference makes Terraform create the shared folder before the NFS export.

## Provider configuration

| Argument | Env var | Required | Description |
|---|---|---|---|
| `endpoint` | `OMV_ENDPOINT` | yes | OMV base URL, e.g. `http://omv.example.com` |
| `username` | `OMV_USERNAME` | yes | OMV admin username |
| `password` | `OMV_PASSWORD` | yes | OMV admin password (sensitive) |
| `allow_destroy` | — | no | Default `false`. Must be `true` to allow deletes. |
| `apply_timeout_minutes` | `OMV_APPLY_TIMEOUT_MINUTES` | no | Max minutes to wait per deploy. Default `120`. |

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
