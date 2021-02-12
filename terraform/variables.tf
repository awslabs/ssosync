variable "region" {}

variable "name" {}

variable "squad" {}

variable "owner" {}

variable "application" {}

variable "environment" {}

variable "container_definitions" {}

variable "include_groups" {
  type        = string
  description = "String with which Google Workspaces must be synch, separeted by comman"
  default     = "Change"
}

