variable "region" {}

variable "name" {}

variable "squad" {}

variable "owner" {}

variable "application" {}

variable "environment" {}

variable "container_definitions" {}

variable "include_groups" {
  type        = list(string)
  description = "List with which Google Workspaces must be synch"
  default     = ["Change"]
}

