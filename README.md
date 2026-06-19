# Terraform Provider for OpenMediaVault

A Terraform provider to manage [OpenMediaVault](https://www.openmediavault.org/)
shared folders and NFS exports declaratively, via OMV's JSON-RPC API.

> Tested against OpenMediaVault 7 (Sandworm).

## Features

- `omv_shared_folder` — manage shared folders (create/read/update/delete/import)
- `omv_nfs_share` — manage NFS exports
- `omv_filesystems` (data source) — discover filesystems for `mntentref`
- **Destroy safety guard** — `allow_destroy = false` (default) makes the provider
  refuse to delete shares, even on `terraform destroy`
- Handles OMV's slow apply by using the background apply + poll mechanism

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

The `sharedfolderref` reference makes Terraform always create the shared folder
before the NFS export that uses it.

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
