resource "smtp_send_mail" "this" {
  to      = "to@example.com"
  from    = "from@example.com"
  subject = "First Terraform plugin"
  body    = "My first mail goes good."
}
