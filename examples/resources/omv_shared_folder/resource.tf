# Look up the filesystem to host the shared folder.
data "omv_filesystems" "all" {}

resource "omv_shared_folder" "example" {
  name      = "example"
  comment   = "example share"
  mntentref = data.omv_filesystems.all.filesystems[0].uuid
}
