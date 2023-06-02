# Configuration-based authentication
provider "smtp" {
  username = "test"
  password = "test123"
  host     = "smtp.example.com"
  port     = "587"
}

# Configuration authentication-less
provider "smtp" {
  authentication  = false
  host            = "smtp.example.com"
  port            = "25"
}