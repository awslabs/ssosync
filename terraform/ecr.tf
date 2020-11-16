resource "aws_ecr_repository" "this" {
  name = local.tags.Name

  image_scanning_configuration {
    scan_on_push = true
  }
}

