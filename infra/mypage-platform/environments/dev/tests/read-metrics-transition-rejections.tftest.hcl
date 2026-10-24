mock_provider "google" {}

variables {
  runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
    read_metrics_enabled       = true
    read_metrics_from_revision = "mypage-api-before"
  })
}

run "reject_transition_while_disabled" {
  command = plan
  variables {
    stage = "disabled"
  }
  expect_failures = [var.runtime_settings]
}

run "reject_transition_on_foundation" {
  command = plan
  variables {
    stage = "foundation"
  }
  expect_failures = [var.runtime_settings]
}

run "reject_other_service_revision" {
  command = plan
  variables {
    stage = "runtime_private"
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
      read_metrics_enabled       = true
      read_metrics_from_revision = "other-api-before"
    })
  }
  expect_failures = [var.runtime_settings]
}
