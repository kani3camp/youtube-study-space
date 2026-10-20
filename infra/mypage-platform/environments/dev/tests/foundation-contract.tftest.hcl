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
