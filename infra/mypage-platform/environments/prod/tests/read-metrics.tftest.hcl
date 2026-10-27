# Provider mocks only. These identifiers and digest are synthetic.
mock_provider "google" {}

variables {
  stage = "runtime_private"
  runtime_settings = {
    image_digest                          = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    project_number                        = "906336399194"
    web_app_id                            = "1:906336399194:web:synthetic"
    public_origin                         = "https://moku.work"
    privacy_version                       = "synthetic-p"
    terms_version                         = "synthetic-t"
    oauth_client_id_version               = "1"
    oauth_client_secret_version           = "2"
    youtube_api_key_version               = "3"
    maintenance_operator_enabled          = true
    maintenance_operator_image_digest     = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    maintenance_operator_image_source_sha = "cccccccccccccccccccccccccccccccccccccccc"
  }
}

run "production_metrics_omitted_by_default" {
  command = plan
  assert {
    condition     = length(module.platform[0].read_metrics_env_values) == 0 && module.platform[0].max_instances == 3 && !module.platform[0].public_invoker_enabled
    error_message = "Production must omit metrics and retain its capacity and invoker boundary."
  }
}

run "production_metrics_explicit_false_is_off" {
  command = plan
  variables {
    runtime_settings = merge({
      image_digest                          = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number                        = "906336399194"
      web_app_id                            = "1:906336399194:web:synthetic"
      public_origin                         = "https://moku.work"
      privacy_version                       = "synthetic-p"
      terms_version                         = "synthetic-t"
      oauth_client_id_version               = "1"
      oauth_client_secret_version           = "2"
      youtube_api_key_version               = "3"
      maintenance_operator_enabled          = true
      maintenance_operator_image_digest     = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      maintenance_operator_image_source_sha = "cccccccccccccccccccccccccccccccccccccccc"
    }, { read_metrics_enabled = false })
  }
  assert {
    condition     = length(module.platform[0].read_metrics_env_values) == 0
    error_message = "Production false must omit the metrics env."
  }
}

run "production_refuses_metrics_in_disabled" {
  command = plan
  variables {
    stage = "disabled"
    runtime_settings = merge({
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:synthetic"
      public_origin               = "https://moku.work"
      privacy_version             = "synthetic-p"
      terms_version               = "synthetic-t"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }, { read_metrics_enabled = true })
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_metrics_in_foundation_metadata" {
  command = plan
  variables {
    stage = "foundation_metadata"
    runtime_settings = merge({
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:synthetic"
      public_origin               = "https://moku.work"
      privacy_version             = "synthetic-p"
      terms_version               = "synthetic-t"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }, { read_metrics_enabled = true })
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_metrics_in_foundation" {
  command = plan
  variables {
    stage = "foundation"
    runtime_settings = merge({
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:synthetic"
      public_origin               = "https://moku.work"
      privacy_version             = "synthetic-p"
      terms_version               = "synthetic-t"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }, { read_metrics_enabled = true })
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_metrics_in_runtime_private" {
  command = plan
  variables {
    stage = "runtime_private"
    runtime_settings = merge({
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:synthetic"
      public_origin               = "https://moku.work"
      privacy_version             = "synthetic-p"
      terms_version               = "synthetic-t"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }, { read_metrics_enabled = true })
  }
  expect_failures = [var.runtime_settings]
}

run "production_refuses_metrics_in_runtime_public" {
  command = plan
  variables {
    stage = "runtime_public"
    runtime_settings = merge({
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:synthetic"
      public_origin               = "https://moku.work"
      privacy_version             = "synthetic-p"
      terms_version               = "synthetic-t"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }, { read_metrics_enabled = true })
  }
  expect_failures = [var.runtime_settings]
}
