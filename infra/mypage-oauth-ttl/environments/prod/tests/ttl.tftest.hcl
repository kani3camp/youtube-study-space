mock_provider "google" {}

variables {
  project_id  = "demo-mypage-ttl-production"
  database_id = "(default)"
}

run "default_database_scope" {
  command = plan
  assert {
    condition = output.scope == {
      environment = "production"
      projectID   = "demo-mypage-ttl-production"
      databaseID  = "(default)"
      collection  = "oauth-transactions"
      field       = "expiresAt"
    }
    error_message = "The root must bind only the production OAuth expiry field."
  }
  assert {
    condition = (
      module.oauth_transaction_ttl.policy.project == output.scope.projectID &&
      module.oauth_transaction_ttl.policy.database == output.scope.databaseID &&
      module.oauth_transaction_ttl.policy.collection == output.scope.collection &&
      module.oauth_transaction_ttl.policy.field == output.scope.field &&
      module.oauth_transaction_ttl.policy.ttl_enabled &&
      module.oauth_transaction_ttl.policy.deletion_policy == "PREVENT" &&
      !module.oauth_transaction_ttl.policy.skip_wait &&
      length(module.oauth_transaction_ttl.policy.index_config) == 0
    )
    error_message = "TTL must be enabled without overriding indexes or allowing destroy."
  }
}

run "explicit_named_database" {
  command = plan
  variables {
    database_id = "demo-mypage-prod-db"
  }
  assert {
    condition     = module.oauth_transaction_ttl.policy.database == "demo-mypage-prod-db"
    error_message = "The explicitly selected existing database must be retained."
  }
}

run "reject_missing_project" {
  command = plan
  variables {
    project_id = ""
  }
  expect_failures = [var.project_id]
}

run "reject_invalid_database" {
  command = plan
  variables {
    database_id = "../wrong"
  }
  expect_failures = [var.database_id]
}
