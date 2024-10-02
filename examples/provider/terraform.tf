terraform {
  # see https://developer.hashicorp.com/terraform/language/settings#specifying-provider-requirements
  required_providers {
    pathfinder = {
      source = "hashicorp-dev-advocates/pathfinder"
      #version = ">= 0.0.0, < 1.0.0"
    }
  }

  # see https://developer.hashicorp.com/terraform/language/settings#specifying-a-required-terraform-version
  required_version = ">= 1.8.0, < 2.0.0"
}
