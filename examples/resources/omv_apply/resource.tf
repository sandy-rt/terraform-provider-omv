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

# A single apply that deploys all staged changes once, at the end of the run.
# Referencing the share resources in `triggers` makes it depend on them and
# re-run whenever any of their attributes change.
resource "omv_apply" "this" {
  triggers = {
    app_folder = sha1(jsonencode(omv_shared_folder.app))
    app_nfs    = sha1(jsonencode(omv_nfs_share.app))
  }

  # OMV applies can be slow; wait up to 90 minutes by default.
  timeout_minutes = 90
}
