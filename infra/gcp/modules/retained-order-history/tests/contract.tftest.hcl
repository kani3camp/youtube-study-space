mock_provider "google" {}

variables {
  project_id  = "test-youtube-study-space"
  dataset_id  = "firestore_export"
  field_order = ["seat_id", "ordered_at", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"]
}

run "preserve_existing_development_schema" {
  command = plan
  assert {
    condition     = jsondecode(google_bigquery_table.retained.schema) == jsondecode(file("${path.module}/schema.json")) && google_bigquery_table.retained.deletion_protection
    error_message = "Existing callers must retain the exact eight-field schema and deletion protection."
  }
}

run "preserve_observed_production_schema_order" {
  command = plan
  variables {
    project_id  = "youtube-study-space"
    field_order = ["ordered_at", "seat_id", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"]
  }
  assert {
    condition     = tolist([for field in jsondecode(google_bigquery_table.retained.schema) : field.name]) == var.field_order && google_bigquery_table.retained.project == "youtube-study-space" && google_bigquery_table.retained.deletion_protection
    error_message = "Import inputs must retain production's observed field order, project and deletion protection."
  }
  assert {
    condition     = tolist([for field in jsondecode(google_bigquery_table.retained.schema)[5].fields : field.name]) == tolist(["namespace", "app", "path", "kind", "name", "id"]) && jsondecode(google_bigquery_table.retained.schema)[0].type == "TIMESTAMP" && jsondecode(google_bigquery_table.retained.schema)[1].type == "INTEGER"
    error_message = "Environment field order must preserve timestamp, seat ID and nested key semantics."
  }
}

run "reject_unknown_field" {
  command = plan
  variables {
    field_order = ["unknown", "ordered_at", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"]
  }
  expect_failures = [var.field_order]
}

run "reject_duplicate_field" {
  command = plan
  variables {
    field_order = ["ordered_at", "ordered_at", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"]
  }
  expect_failures = [var.field_order]
}

run "reject_missing_field" {
  command = plan
  variables {
    field_order = ["seat_id", "ordered_at", "user_id", "menu_code", "is_member_seat", "__key__", "__error__"]
  }
  expect_failures = [var.field_order]
}
