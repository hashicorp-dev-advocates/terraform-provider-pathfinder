# Terraform Provider: Pathfinder

> [!CAUTION]
> HashiCorp Forge is a place where we share experimental software. These providers are rapidly evolving and might change in breaking ways. We can't guarantee their stability or that they'll be consistently available. We recommend using this Terraform Provider for experimentation and feedback only.

The [Pathfinder Terraform provider]((https://registry.terraform.io/providers/hashicorp-dev-advocates/pathfinder/latest/docs)) allows you to interact with the Pathfinder API. The Pathfinder API provides movement planning for the Wave Rover, an unmanned ground vehicle (UGV) developed by Waveshare.

* [Terraform Registry](https://registry.terraform.io/providers/hashicorp-dev-advocates/pathfinder/latest/docs)

## Usage

To use the Pathfinder Terraform provider, declare the provider as a required provider in your Terraform configuration:

```hcl
terraform {
  required_providers {
    pathfinder = {
      source = "hashicorp-dev-advocates/pathfinder"
    }
  }
}
```

### Configuring the Pathfinder provider

To configure the Pathfinder provider, specify the address of the Pathfinder API that's running on the Wave Rover:

```hcl
provider "pathfinder" {
  address = "http://192.168.4.1:80"
  api_key = "your-api-key"
}
```

### Managing movement

To instruct the device to move in a specific direction, use the `pathfinder_movement` resource. The following example moves the device forward and then right twice:

```hcl
resource "pathfinder_movement" "example" {
  name = "example"
  steps {
    angle     = 0
    direction = "forward"
    distance  = 1
  }

  steps {
    angle     = 90
    direction = "right"
    distance  = 1
  }

  steps {
    angle     = 90
    direction = "right"
    distance  = 1
  }
}
```

## Contributing

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21

## Building The Provider

To compile the provider, run `make install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

```shell
make install
```

## Using the provider locally

To use the provider locally, make sure to build the provider first. Then, create a `.terraformrc` file in your home directory with the following content:

```hcl
provider_installation {
    dev_overrides {
        "hashicorp-dev-advocates/pathfinder" = "/Users/username/go/bin"
    }

    direct {}
}
```

This instructs `terraform` to use the locally built provider instead of the one from the Terraform Registry.

## Running the acceptance tests

To run the acceptance tests, use the following command:

```shell
make testacc
```

## Linting and formatting the code

We use [golangci-lint](https://golangci-lint.run/) to lint the Go code. To run the linter, use the following command:

```shell
make lint
```

To format the code, use the following command:

```shell
make fmt
```

## Linting acceptance tests configurations

To lint the acceptance tests configurations, you’ll need to have [terrafmt](https://github.com/katbyte/terrafmt) installed. Once installed, run the following command to lint:

```shell
make tests-lint-fix
```


## Generating the documentation

Documentation is generated with tfplugindocs. To generate the documentation, run the following command:

```shell
make docs
```

## Adding copyright headers

To add copyright headers to the source files, you’ll need to have copywrite installed. Once installed, run the following command to add the headers:

```shell
make copyright
```
