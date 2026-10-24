# Production rejects transition declarations even when metrics are false.
mock_provider "google" {}

variables {
  runtime_settings = {
    image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    project_number              = "123456789012"
    web_app_id                  = "1:123456789012:web:synthetic"
    public_origin               = "https://example.test"
    privacy_version             = "synthetic-p"
    terms_version               = "synthetic-t"
    oauth_client_id_version     = "1"
    oauth_client_secret_version = "2"
    youtube_api_key_version     = "3"
    read_metrics_enabled        = false
    read_metrics_from_revision  = "mypage-api-before"
  }
}

run "production_refuses_transition_in_disabled" {
  command = plan
  variables {
    stage = "disabled"
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_transition_in_foundation_metadata" {
  command = plan
  variables {
    stage = "foundation_metadata"
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_transition_in_foundation" {
  command = plan
  variables {
    stage = "foundation"
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_transition_in_runtime_private" {
  command = plan
  variables {
    stage = "runtime_private"
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_transition_in_runtime_public" {
  command = plan
  variables {
    stage = "runtime_public"
  }
  expect_failures = [var.runtime_settings]
}
