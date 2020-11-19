locals {
  tags = {
    Name        = var.name
    Squad       = var.squad
    Owner       = var.owner
    Environment = var.environment
    Application = var.application
    Terraform   = "true"
  }
}
