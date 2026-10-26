# Every case uses provider mocks. No ADC, project API, Secret Manager or S3 State calls.
mock_provider "google" {}

run "disabled_by_default" {
  command = plan
  assert {
    condition     = length(module.platform) == 0 && output.stage == "disabled"
    error_message = "Default must create no MyPage resources."
  }
}

run "foundation_is_private" {
  command = plan
  variables {
    stage = "foundation"
  }
  assert {
    condition     = length(module.platform) == 1 && !module.platform[0].runtime_enabled && !module.platform[0].public_invoker_enabled && module.platform[0].history_coverage_env_count == 0
    error_message = "Foundation must not deploy an API or public invoker."
  }
}

run "reject_invalid_stage" {
  command = plan
  variables {
    stage = "unreviewed"
  }
  expect_failures = [var.stage]
}

run "reject_runtime_without_reviewed_inputs" {
  command = plan
  variables {
    stage = "runtime_private"
  }
  expect_failures = [var.runtime_settings]
}

run "runtime_private_is_not_public" {
  command = plan
  variables {
    stage = "runtime_private"
    runtime_settings = {
      image_digest                          = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number                        = "123456789012"
      web_app_id                            = "1:123456789012:web:abcdef1234567890"
      public_origin                         = "https://example.test"
      privacy_version                       = "2026-10-18"
      terms_version                         = "2026-10-18"
      oauth_client_id_version               = "1"
      oauth_client_secret_version           = "2"
      youtube_api_key_version               = "3"
      maintenance_operator_enabled          = true
      maintenance_operator_image_digest     = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      maintenance_operator_image_source_sha = "cccccccccccccccccccccccccccccccccccccccc"
      history_coverage_json                 = jsonencode([{ start = "2026-10-01T00:00:00Z", end = "2026-10-02T00:00:00Z" }])
    }
  }
  assert {
    condition     = module.platform[0].runtime_enabled && !module.platform[0].public_invoker_enabled && module.platform[0].max_instances == 3 && module.platform[0].history_coverage_env_count == 1
    error_message = "Private runtime must not grant allUsers."
  }
}

run "public_invoker_requires_explicit_stage" {
  command = plan
  variables {
    stage = "runtime_public"
    runtime_settings = {
      image_digest                          = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number                        = "123456789012"
      web_app_id                            = "1:123456789012:web:abcdef1234567890"
      public_origin                         = "https://example.test"
      privacy_version                       = "2026-10-18"
      terms_version                         = "2026-10-18"
      oauth_client_id_version               = "1"
      oauth_client_secret_version           = "2"
      youtube_api_key_version               = "3"
      maintenance_operator_enabled          = true
      maintenance_operator_image_digest     = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
      maintenance_operator_image_source_sha = "cccccccccccccccccccccccccccccccccccccccc"
    }
  }
  assert {
    condition     = module.platform[0].runtime_enabled && module.platform[0].public_invoker_enabled && module.platform[0].max_instances == 3 && module.platform[0].history_coverage_env_count == 0
    error_message = "Only the explicit public stage can grant allUsers."
  }
}

run "invalid_history_coverage_is_refused_without_cloud_access" {
  command = plan
  variables {
    stage = "disabled"
    runtime_settings = {
      image_digest                = ""
      project_number              = ""
      web_app_id                  = ""
      public_origin               = ""
      privacy_version             = ""
      terms_version               = ""
      oauth_client_id_version     = ""
      oauth_client_secret_version = ""
      youtube_api_key_version     = ""
      history_coverage_json       = "{not-json}"
    }
  }
  expect_failures = [var.runtime_settings]
}
