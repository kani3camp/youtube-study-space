# Provider mocks only: no credentials, real State or Firestore reads.
mock_provider "google" {}

variables {
  runtime_settings = jsondecode(file("tests/runtime-settings.fixture.json"))
}

run "metrics_omitted_by_default" {
  command = plan
  assert {
    condition     = length(module.platform.read_metrics_env_values) == 0 && module.platform.max_instances == 1 && !module.platform.public_invoker_enabled
    error_message = "Existing DEV settings must omit the metrics env and preserve private capacity."
  }
}

run "metrics_explicit_false_is_off" {
  command = plan
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = false })
  }
  assert {
    condition     = length(module.platform.read_metrics_env_values) == 0
    error_message = "Explicit false must omit the env, not create a false-valued revision setting."
  }
}

run "metrics_opt_in_is_fixed_true_on_private_runtime" {
  command = plan
  variables {
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = true })
  }
  assert {
    condition     = jsonencode(module.platform.read_metrics_env_values) == jsonencode(["true"]) && module.platform.max_instances == 1 && !module.platform.public_invoker_enabled
    error_message = "DEV opt-in must add exactly one literal true env without changing capacity or invoker."
  }
}

run "metrics_opt_in_preserves_adopted_public_invoker" {
  command = plan
  variables {
    public_invoker   = true
    runtime_settings = merge(jsondecode(file("tests/runtime-settings.fixture.json")), { read_metrics_enabled = true })
  }
  assert {
    condition     = jsonencode(module.platform.read_metrics_env_values) == jsonencode(["true"]) && module.platform.max_instances == 1 && module.platform.public_invoker_enabled
    error_message = "Metrics must not change the adopted public invoker boundary."
  }
}
