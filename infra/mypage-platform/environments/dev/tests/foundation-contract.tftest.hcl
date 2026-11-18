# The deployment guard consumes this real native Plan JSON in credentialless CI.
# There is no stage: the declared set follows only the inputs CI derives from
# facts outside Terraform (runtime settings, and the owner's public binding).
mock_provider "google" {}

run "base_contract" {
  command = plan
  assert {
    condition     = !module.platform.runtime_enabled && !module.platform.public_invoker_enabled
    error_message = "Without runtime settings only the base resources may be declared."
  }
}

run "runtime_contract" {
  command = plan
  variables {
    runtime_settings = jsondecode(file("tests/runtime-settings.fixture.json"))
  }
  assert {
    condition     = module.platform.runtime_enabled && !module.platform.public_invoker_enabled
    error_message = "Runtime settings declare the runtime but never the owner-managed public invoker."
  }
}

run "public_invoker_contract" {
  command = plan
  variables {
    runtime_settings = jsondecode(file("tests/runtime-settings.fixture.json"))
    public_invoker   = true
  }
}
