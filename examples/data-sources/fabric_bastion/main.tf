# Resolve the caller's bastion host and login from the FABRIC token.
data "fabric_bastion" "me" {}

output "bastion_ssh" {
  value = "${data.fabric_bastion.me.username}@${data.fabric_bastion.me.host}"
}
