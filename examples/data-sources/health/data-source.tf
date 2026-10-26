data "pathfinder_health" "example" {}

output "healthy" {
  value = data.pathfinder_health.example.healthy
}
