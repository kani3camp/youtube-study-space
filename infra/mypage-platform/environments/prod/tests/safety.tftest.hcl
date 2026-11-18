# Every case uses provider mocks. No ADC, project API, Secret Manager or S3 State calls.
mock_provider "google" {}

run "base_only_without_runtime_settings" {
  command = plan
  assert {
    condition     = !module.platform.runtime_enabled && !module.platform.public_invoker_enabled && output.service_url == null
    error_message = "Without runtime settings no API, Job or public invoker may be declared."
  }
}

run "reject_public_invoker_without_runtime" {
  command = plan
  variables {
    public_invoker = true
  }
  expect_failures = [var.public_invoker]
}

run "reject_runtime_without_reviewed_inputs" {
  command = plan
  variables {
    runtime_settings = {
      image_digest                = ""
      project_number              = ""
      web_app_id                  = ""
      public_origin               = ""
      oauth_client_id_version     = ""
      oauth_client_secret_version = ""
      youtube_api_key_version     = ""
    }
  }
  expect_failures = [var.runtime_settings]
}

run "reject_foreign_project_number" {
  command = plan
  variables {
    runtime_settings = {
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "48101442817"
      web_app_id                  = "1:48101442817:web:abcdef1234567890"
      public_origin               = "https://moku.work"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }
  }
  expect_failures = [var.runtime_settings]
}

run "runtime_is_not_public_by_default" {
  command = plan
  variables {
    runtime_settings = {
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:abcdef1234567890"
      public_origin               = "https://moku.work"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }
  }
  assert {
    condition     = module.platform.runtime_enabled && !module.platform.public_invoker_enabled && module.platform.max_instances == 3
    error_message = "Runtime settings alone must not grant allUsers."
  }
}

run "public_invoker_requires_explicit_adoption" {
  command = plan
  variables {
    runtime_settings = {
      image_digest                = "asia-northeast2-docker.pkg.dev/youtube-study-space/mypage/mypage-api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      project_number              = "906336399194"
      web_app_id                  = "1:906336399194:web:abcdef1234567890"
      public_origin               = "https://moku.work"
      oauth_client_id_version     = "1"
      oauth_client_secret_version = "2"
      youtube_api_key_version     = "3"
    }
    public_invoker = true
  }
  assert {
    condition     = module.platform.runtime_enabled && module.platform.public_invoker_enabled && module.platform.max_instances == 3
    error_message = "Only the explicit adoption input may declare allUsers."
  }
}
