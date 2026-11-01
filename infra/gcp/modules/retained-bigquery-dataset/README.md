# Retained BigQuery dataset metadata

This module imports the existing retained dataset without acquiring dataset ACL/IAM ownership. Inline access, dataset IAM/access resources, authorized views/datasets and project IAM are outside this module. The pinned provider treats access as optional/computed; omission plus ignore_changes for access preserves all existing entries. Access must still be compared by fresh metadata before/after each import; it is intentionally not an ACL drift detector.

The observed dev/prod datasets are DEFAULT datasets with no description, labels, default table/partition expiration, collation, external reference or CMEK; max time travel is 168 hours. Storage billing model is absent in the REST response and remains the provider's optional/computed value rather than being forced to an improvement value. Project and location are environment inputs (dev asia-southeast2 / prod asia-northeast2). Production does not instantiate or import this module in this wave.

delete_contents_on_destroy=false is the provider's import default, not a remote dataset setting. prevent_destroy and the unchanged CI import-only gate protect retained contents. The provider's local deletion policy import default is preserved; no destructive policy change is bundled with ownership.

Dataset Read performs a single metadata GET and needs bigquery.datasets.get only. No table data read, query/job, list, mutation or production permissions are required. See the [pinned provider source](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/bigquery/resource_bigquery_dataset.go).

Run credentialless root fmt/init/validate and repository CI first. After merging to the trusted integration ref, require an import-only authenticated plan, separate approved same-SHA re-plan/apply, post-apply no-op, unchanged live metadata/access, S3 state/lock verification and public log/Summary/artifact audit before starting the dependent table wave.
