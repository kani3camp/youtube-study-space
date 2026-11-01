# Retained order-history table

The same eight field definitions and nested key fields are shared across environments; project/dataset and top-level field order are inputs. Development starts seat_id/ordered_at; production starts ordered_at/seat_id. The import configuration preserves the exact development order. Production has no module instance/import in this wave.

Fresh metadata shows TABLE, not view/materialized/external. No description, labels, partitioning, clustering, expiration or CMEK is configured in either environment. This module adds no IAM ownership. Schema is explicitly represented rather than ignored, and is compared with provider-canonical state after import. No data query/read is needed.

The pinned provider imports deletion_protection=true; this local guard and prevent_destroy are retained alongside import-only CI. Provider local deletion_policy remains the import default; remote data/metadata are not improved during migration.

Read calls tables.get (bigquery.tables.get) only, without tabledata or jobs APIs. [Pinned provider Read/schema handling](https://github.com/hashicorp/terraform-provider-google/blob/v8.5.0/google/services/bigquery/resource_bigquery_table.go).

Start only after dataset wave CI apply/post-plan/metadata/backend/log checks PASS. Require the same gates for this wave and preserve field order; never apply a schema update/replacement. A read-only development plan with the alternate observed production field order can measure the pinned provider's order suppression; it must never be applied.
