data "pathfinder_ready" "example" {}

output "ready" {
  value = data.pathfinder_ready.example.ready
}
