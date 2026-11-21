# Provider mocks only. These identifiers and digest are synthetic.
mock_provider "google" {}

variables {
  runtime_settings = {
    image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    project_number              = "906336399194"
    web_app_id                  = "1:906336399194:web:synthetic"
    public_origin               = "https://moku.work"
    oauth_client_id_version     = "1"
    oauth_client_secret_version = "2"
    youtube_api_key_version     = "3"
  }
}

run "production_metrics_omitted_by_default" {
  command = plan
  assert {
    condition     = length(module.platform.read_metrics_env_values) == 0 && module.platform.max_instances == 3 && !module.platform.public_invoker_enabled
    error_message = "Production must omit metrics and retain its capacity and invoker boundary."
  }
}

run "production_metrics_explicit_false_is_off" {
  command = plan
  variables {
    runtime_settings = merge({
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:synthetic"
      public_origin               = "https://moku.work"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }, { read_metrics_enabled = false })
  }
  assert {
    condition     = length(module.platform.read_metrics_env_values) == 0
    error_message = "Production false must omit the metrics env."
  }
}

run "production_refuses_metrics" {
  command = plan
  variables {
    runtime_settings = merge({
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:synthetic"
      public_origin               = "https://moku.work"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }, { read_metrics_enabled = true })
  }
  expect_failures = [var.runtime_settings]
}
