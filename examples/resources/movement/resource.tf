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
