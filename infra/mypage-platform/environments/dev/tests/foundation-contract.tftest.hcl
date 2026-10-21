# The deployment guard consumes this real native Plan JSON in credentialless CI.
mock_provider "google" {}

run "foundation_metadata_contract" {
  command = plan
  variables {
    stage = "foundation_metadata"
  }
}

run "foundation_contract" {
  command = plan
  variables {
    stage = "foundation"
  }
}

run "registry_contract" {
  command = plan
  variables {
    stage = "registry"
  }
  assert {
    condition     = !module.platform[0].runtime_enabled && !module.platform[0].public_invoker_enabled
    error_message = "Registry provisioning cannot activate the API."
  }
}

run "runtime_private_contract" {
  command = plan
  variables {
    stage            = "runtime_private"
    runtime_settings = jsondecode(file("tests/runtime-settings.fixture.json"))
  }
}

run "runtime_public_contract" {
  command = plan
  variables {
    stage            = "runtime_public"
    runtime_settings = jsondecode(file("tests/runtime-settings.fixture.json"))
  }
}
