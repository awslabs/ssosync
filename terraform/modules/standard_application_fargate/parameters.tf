module "parameter_store_ssosync_google_credentials" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_GOOGLE_CREDENTIALS"
  ssm_parameter_description = "Name of file that contains Google service account"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "change"
}

module "parameter_store_ssosync_google_credentials_secret" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_GOOGLE_CREDENTIALS_SECRET"
  ssm_parameter_description = "Google service account base64 coded"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "change"
}

module "parameter_store_ssosync_log_level" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_LOG_LEVEL"
  ssm_parameter_description = "Log level used by app"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "INFO"
}

module "parameter_store_ssosync_google_admin" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_GOOGLE_ADMIN"
  ssm_parameter_description = "Google e-mail account used by service account"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "change"
}

module "parameter_store_ssosync_scim_endpoint" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_SCIM_ENDPOINT"
  ssm_parameter_description = "SCIM endpoint provided by AWS SSO"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "change"
}

module "parameter_store_ssosync_scim_access_token" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_SCIM_ACCESS_TOKEN"
  ssm_parameter_description = "SCIM access token provided by AWS SSO"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "change"
}

module "parameter_store_ssosync_include_groups" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_INCLUDE_GROUPS"
  ssm_parameter_description = "Groups from gsuite that will be sync to AWS SSO"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = var.include_groups
}

module "parameter_store_ssosync_log_format" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/SSOSYNC_LOG_FORMAT"
  ssm_parameter_description = "Log format generated by application"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "json"
}

module "parameter_store_cooldown_time" {
  source = "git::ssh://git@github.com/creditas/terraform-modules.git//ssm/secure?ref=1.0.2"

  name        = var.name
  squad       = var.squad
  owner       = var.owner
  environment = var.environment
  application = var.name

  ssm_parameter_name        = "/${var.name}/${var.environment}/COOLDOWN_TIME"
  ssm_parameter_description = "Amount of time that application will slep between executions"
  ssm_parameter_type        = "SecureString"

  ssm_parameter_value = "3600"
}
