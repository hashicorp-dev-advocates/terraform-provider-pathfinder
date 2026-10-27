# Uploads the plan to the device. The device does not move.
resource "pathfinder_movement_plan" "example" {
  name = "example"

  steps {
    angle     = 0
    direction = "forward"
    distance  = 1
  }

  steps {
    angle     = 90
    direction = "forward"
    distance  = 1
  }

  # Run the plan once it has been uploaded.
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.pathfinder_run_movement.example]
    }
  }
}

# Runs the plan that is stored on the device.
action "pathfinder_run_movement" "example" {}
