data "pathfinder_health" "example" {}

output "ready" {
  value = data.pathfinder_ready.example.ready
}
