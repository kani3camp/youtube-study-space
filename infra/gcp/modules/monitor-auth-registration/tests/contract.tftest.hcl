mock_provider "google" {
  mock_resource "google_cloudfunctions2_function" {
    defaults = {
      service_config = {
        uri     = "https://registration-guard.example.run.app"
        service = "projects/demo-monitor-auth/locations/asia-southeast2/services/monitor-password-signup-guard"
      }
    }
  }
}
variables {
  project_id                    = "demo-monitor-auth"
  region                        = "asia-southeast2"
  runtime_service_account_email = "monitor-auth-runtime@demo-monitor-auth.iam.gserviceaccount.com"
  build_service_account_email   = "monitor-auth-build@demo-monitor-auth.iam.gserviceaccount.com"
}
run "registration_only_and_existing_login_preserved" {
  command = plan
  assert {
    condition     = google_identity_platform_config.auth.sign_in[0].email[0].enabled && google_identity_platform_config.auth.sign_in[0].email[0].password_required
    error_message = "既存のメール・パスワードログインを維持する必要があります。"
  }
  assert {
    condition     = length(google_identity_platform_config.auth.blocking_functions[0].triggers) == 1 && one(google_identity_platform_config.auth.blocking_functions[0].triggers).event_type == "beforeCreate"
    error_message = "新規登録だけに接続し、beforeSignInでログインを止めてはいけません。"
  }
  assert {
    condition     = google_storage_bucket.source.public_access_prevention == "enforced" && google_storage_bucket.source.uniform_bucket_level_access && google_storage_bucket_object.source.deletion_policy == "ABANDON"
    error_message = "ソースは非公開・改版ごとの名前とし、復旧用の旧版を保持する必要があります。"
  }
  assert {
    condition     = google_artifact_registry_repository_iam_member.builder.repository == "monitor-auth" && google_artifact_registry_repository_iam_member.builder.role == "roles/artifactregistry.writer" && google_artifact_registry_repository_iam_member.builder.member == "serviceAccount:${var.build_service_account_email}"
    error_message = "ビルド権限は専用リポジトリに直接付与してください。"
  }
}
run "preserve_existing_before_sign_in" {
  command = apply
  variables { preserved_before_sign_in_uri = "https://existing-signin.example.run.app" }
  assert {
    condition     = length(google_identity_platform_config.auth.blocking_functions[0].triggers) == 2 && local.triggers.beforeSignIn == "https://existing-signin.example.run.app"
    error_message = "既存のbeforeSignInを無効化・差し替えしてはいけません。"
  }
}
run "reject_other_project_runtime" {
  command = plan
  variables { runtime_service_account_email = "monitor-runtime@other-project.iam.gserviceaccount.com" }
  expect_failures = [var.runtime_service_account_email]
}
run "reject_shared_build_and_runtime" {
  command = plan
  variables { build_service_account_email = "monitor-auth-runtime@demo-monitor-auth.iam.gserviceaccount.com" }
  expect_failures = [var.build_service_account_email]
}
