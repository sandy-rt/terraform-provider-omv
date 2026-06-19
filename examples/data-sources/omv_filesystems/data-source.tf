data "omv_filesystems" "all" {}

output "filesystems" {
  value = data.omv_filesystems.all.filesystems
}
