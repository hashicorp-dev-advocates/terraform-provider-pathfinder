data "pathfinder_wifi_networks" "example" {}

output "wifi_networks" {
  value = data.pathfinder_wifi_networks.example.networks
}
