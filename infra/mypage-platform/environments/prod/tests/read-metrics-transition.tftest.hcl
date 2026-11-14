# Production rejects transition declarations even when metrics are false.
mock_provider "google" {}

variables {
  runtime_settings = {
    image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    project_number              = "906336399194"
    web_app_id                  = "1:906336399194:web:synthetic"
    public_origin               = "https://moku.work"
    privacy_version             = "synthetic-p"
    terms_version               = "synthetic-t"
    oauth_client_id_version     = "1"
    oauth_client_secret_version = "2"
    youtube_api_key_version     = "3"
    read_metrics_enabled        = false
    read_metrics_from_revision  = "mypage-api-before"
  }
}

run "production_refuses_transition_in_runtime_private" {
  command = plan
  variables {
    stage = "runtime_private"
  }
  expect_failures = [var.runtime_settings]
}
