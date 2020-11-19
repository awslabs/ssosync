terraform {
  required_version = "~> 0.12"

  backend "s3" {}
}

provider "aws" {
  version = "~> 2.70.0"
  region  = var.region
}

