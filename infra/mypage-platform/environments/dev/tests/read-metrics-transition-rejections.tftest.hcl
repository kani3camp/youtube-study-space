mock_provider "google" {}

variables {
  runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
    read_metrics_enabled       = true
    read_metrics_from_revision = "mypage-api-before"
  })
}

run "reject_other_service_revision" {
  command = plan
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), {
      read_metrics_enabled       = true
      read_metrics_from_revision = "other-api-before"
    })
  }
  expect_failures = [var.runtime_settings]
}
