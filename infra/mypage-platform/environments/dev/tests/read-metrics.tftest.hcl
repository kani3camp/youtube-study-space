# Provider mocks only: no credentials, real State or Firestore reads.
mock_provider "google" {}

variables {
  stage            = "runtime_private"
  runtime_settings = jsondecode(file("tests/runtime-settings.fixture.json"))
}

run "metrics_omitted_by_default" {
  command = plan
  assert {
    condition     = length(module.platform[0].read_metrics_env_values) == 0 && module.platform[0].max_instances == 1 && !module.platform[0].public_invoker_enabled
    error_message = "Existing DEV settings must omit the metrics env and preserve private capacity."
  }
}

run "metrics_explicit_false_is_off" {
  command = plan
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = false })
  }
  assert {
    condition     = length(module.platform[0].read_metrics_env_values) == 0
    error_message = "Explicit false must omit the env, not create a false-valued revision setting."
  }
}

run "metrics_opt_in_is_fixed_true_on_private_runtime" {
  command = plan
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = true })
  }
  assert {
    condition     = jsonencode(module.platform[0].read_metrics_env_values) == jsonencode(["true"]) && module.platform[0].max_instances == 1 && !module.platform[0].public_invoker_enabled
    error_message = "DEV opt-in must add exactly one literal true env without changing capacity or invoker."
  }
}

run "metrics_opt_in_preserves_explicit_public_stage" {
  command = plan
  variables {
    stage            = "runtime_public"
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = true })
  }
  assert {
    condition     = jsonencode(module.platform[0].read_metrics_env_values) == jsonencode(["true"]) && module.platform[0].max_instances == 1 && module.platform[0].public_invoker_enabled
    error_message = "Metrics must not change the explicitly selected public runtime boundary."
  }
}

run "metrics_opt_in_does_not_deploy_foundation" {
  command = plan
  variables {
    stage            = "foundation_metadata"
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = true })
  }
  assert {
    condition     = !module.platform[0].runtime_enabled && !module.platform[0].public_invoker_enabled && length(module.platform[0].read_metrics_env_values) == 0
    error_message = "Metrics must not create a runtime or public invoker during foundation preparation."
  }
}

run "metrics_opt_in_does_not_activate_disabled_root" {
  command = plan
  variables {
    stage            = "disabled"
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = true })
  }
  assert {
    condition     = length(module.platform) == 0 && !output.runtime_enabled
    error_message = "Metrics must not create resources in a disabled root."
  }
}
