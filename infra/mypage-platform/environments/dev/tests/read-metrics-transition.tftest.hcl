# Only provider mocks. Apply seeds test state; it never accesses a cloud project.
mock_provider "google" {
  mock_resource "google_cloud_run_v2_service" {
    defaults = {
      generation              = "7"
      observed_generation     = "7"
      update_time             = "2026-10-23T00:00:00Z"
      latest_ready_revision   = "mypage-api-before"
      latest_created_revision = "mypage-api-before"
      reconciling             = false
      terminal_condition = [{
        state = "CONDITION_SUCCEEDED"
      }]
      traffic_statuses = [{
        type     = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
        percent  = 100
        revision = "mypage-api-before"
        tag      = ""
      }]
    }
  }
}

variables {
  stage = "runtime_private"
  runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
    read_metrics_enabled       = false
    read_metrics_from_revision = ""
  })
}

run "seed_metrics_off" {
  command = apply
}

run "enable_metrics_only" {
  command = plan
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
      read_metrics_enabled       = true
      read_metrics_from_revision = "mypage-api-before"
    })
  }
  assert {
    condition     = length(module.platform[0].read_metrics_env_values) == 1 && module.platform[0].read_metrics_env_values[0] == "true"
    error_message = "An explicit DEV enable must emit one metrics environment entry."
  }
}

run "seed_metrics_on" {
  command = apply
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
      read_metrics_enabled       = true
      read_metrics_from_revision = ""
    })
  }
}

run "disable_metrics_only" {
  command = plan
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
      read_metrics_enabled       = false
      read_metrics_from_revision = "mypage-api-before"
    })
  }
  assert {
    condition     = length(module.platform[0].read_metrics_env_values) == 0
    error_message = "An explicit DEV disable must remove the metrics environment entry."
  }
}
