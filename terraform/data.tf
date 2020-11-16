data "aws_caller_identity" "current" {}

data "aws_region" "current" {}

data "aws_vpc" "this" {
  filter {
    name   = "tag:Main"
    values = ["true"]
  }
}

data "aws_subnet_ids" "private" {
  vpc_id = data.aws_vpc.this.id

  filter {
    name   = "tag:Tier"
    values = ["private"]
  }
}

