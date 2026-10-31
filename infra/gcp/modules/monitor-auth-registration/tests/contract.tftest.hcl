mock_provider "google" {}
variables {
  project_id                    = "demo-monitor-auth"
  region                        = "asia-southeast2"
  runtime_service_account_email = "monitor-auth-runtime@demo-monitor-auth.iam.gserviceaccount.com"
  build_service_account_email   = "monitor-auth-build@demo-monitor-auth.iam.gserviceaccount.com"
}
run "registration_guard_detached_and_existing_login_preserved" {
  command = plan
  assert {
    condition     = google_identity_platform_config.auth.sign_in[0].email[0].enabled && google_identity_platform_config.auth.sign_in[0].email[0].password_required
    error_message = "既存のメール・パスワードログインを維持する必要があります。"
  }
  assert {
    condition     = length(google_identity_platform_config.auth.blocking_functions) == 0
    error_message = "登録制限の撤去ではbeforeCreateを解除し、ログイン用のAuth設定を維持してください。"
  }
  assert {
    condition     = google_storage_bucket.source.public_access_prevention == "enforced" && google_storage_bucket.source.uniform_bucket_level_access
    error_message = "撤去中の専用バケットは、削除完了まで非公開を維持してください。"
  }

}
run "preserve_existing_before_sign_in" {
  command = apply
  variables { preserved_before_sign_in_uri = "https://existing-signin.example.run.app" }
  assert {
    condition     = length(google_identity_platform_config.auth.blocking_functions[0].triggers) == 1 && local.triggers.beforeSignIn == "https://existing-signin.example.run.app"
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
