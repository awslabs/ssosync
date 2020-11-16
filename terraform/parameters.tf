module "parameter_store_ssosync_google_credentials" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_GOOGLE_CREDENTIALS"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "credentials.json"
}

module "parameter_store_ssosync_google_credentials_secret" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_GOOGLE_CREDENTIALS_SECRET"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "credentials.json"
}

module "parameter_store_ssosync_log_level" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_LOG_LEVEL"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "debug"
}

module "parameter_store_ssosync_google_admin" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_GOOGLE_ADMIN"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "aws-auth@creditas.com"
}

module "parameter_store_ssosync_SCIM_ENDPOINT" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_SCIM_ENDPOINT"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "https://scim.us-east-1.amazonaws.com/f3va94ba79f-ddec-4ab1-92c6-64e9f8efef82/scim/v2/"
}

module "parameter_store_ssosync_scim_access_token" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_SCIM_ACCESS_TOKEN"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "1505a637-19e4-4c3e-9333-1977c4b5244e:762fc824-2851-45b2-81da-d5733273e5f1:3oXZdBMtIx9eAh7nVQx1Y6smsh/1tgRUfY4SKyeIbZb4cXFHucTcH8BX2p4+JbSQqL8RjNCQ478IDE9oJQb87j+aZY4cVyZN5OJFoiTCF4dATCHIoCNfNi5+4Nbjx4lNE4w9r70iKoy9QuVo2gPC46q6ECnzzkEaXhA=:FQ7VuRUKXvD6GovtSoA7rGWYymv0Bk3gGgcbr8mWV+I9/1CVg6o8W36qBfN7STGsoD5UZPS+o5PEBLFWcqF/LP1BAnPlMs7isLXMWXOZZmoxCXW5V4jMxsMl+HaUoJ4AVunv9SfpMmOLP9RbQB2oRIuHsG2EkDgh00DbJrb6pqkk2NkXy4H3iaLrMVtrUzHv0vk8vM6ze/Iy67MpwkOd2BXU6r0nxJVaW/o7gWAX36dsbakWU+uTVU5GaJeNbKNQrlmCac1pA5vbi5kOKiEqDXXY91i2vNry74b4wQTszxmRl9UZ67309e541CGhmrOhBRaQ+UkCarjvZQ+cwUqSuw=="
}

module "parameter_store_ssosync_include_groups" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_INCLUDE_GROUPS"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "access-aws-scd@creditas.com"
}

module "parameter_store_ssosync_log_format" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = local.tags.Name
  squad       = local.tags.Squad
  owner       = local.tags.Owner
  environment = local.tags.Environment
  application = local.tags.Name

  ssm_parameter_name        = "/${local.tags.Name}/${local.tags.Environment}/SSOSYNC_LOG_FORMAT"
  ssm_parameter_description = "empty"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "json"
}
