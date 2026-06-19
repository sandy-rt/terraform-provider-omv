data "omv_filesystems" "all" {}

resource "omv_shared_folder" "example" {
  name      = "example"
  comment   = "example share"
  mntentref = data.omv_filesystems.all.filesystems[0].uuid
}

resource "omv_nfs_share" "example" {
  sharedfolderref = omv_shared_folder.example.uuid
  client          = "*"
  options         = "rw"
  extraoptions    = "insecure,no_root_squash,subtree_check,sync"
  comment         = "example share"
}
