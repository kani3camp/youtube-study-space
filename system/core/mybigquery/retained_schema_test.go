package mybigquery

import (
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"

	"app.modules/core/repository"
)

func retainedTestSchema(names ...string) bigquery.Schema {
	schema := make(bigquery.Schema, 0, len(names))
	for _, name := range names {
		schema = append(schema, &bigquery.FieldSchema{Name: name})
	}
	return schema
}

func TestRetainedCollectionProjection(t *testing.T) {
	t.Parallel()

	t.Run("user activities preserve temporary table field order", func(t *testing.T) {
		t.Parallel()
		schema := retainedTestSchema(
			"user_id",
			"seat_id",
			"taken_at",
			"activity_type",
			"is_member_seat",
			"__key__",
			"__error__",
			"__has_error__",
		)

		got, err := retainedCollectionProjection(repository.UserActivities, schema)
		if err != nil {
			t.Fatalf("retainedCollectionProjection() error = %v", err)
		}
		want := "`user_id`, `seat_id`, `taken_at`, `activity_type`, `is_member_seat`, `__key__`, `__error__`, `__has_error__`"
		if got != want {
			t.Fatalf("retainedCollectionProjection() = %q, want %q", got, want)
		}
	})

	t.Run("legacy timestamp is rejected instead of silently reintroduced", func(t *testing.T) {
		t.Parallel()
		schema := retainedTestSchema(
			"seat_id",
			"taken_at",
			"timestamp",
			"user_id",
			"activity_type",
			"is_member_seat",
			"__key__",
			"__error__",
			"__has_error__",
		)

		_, err := retainedCollectionProjection(repository.UserActivities, schema)
		if err == nil || !strings.Contains(err.Error(), "unexpected field \"timestamp\"") {
			t.Fatalf("retainedCollectionProjection() error = %v, want unexpected timestamp rejection", err)
		}
	})

	t.Run("missing canonical field is rejected", func(t *testing.T) {
		t.Parallel()
		schema := retainedTestSchema(
			"seat_id",
			"user_id",
			"activity_type",
			"is_member_seat",
			"__key__",
			"__error__",
			"__has_error__",
		)

		_, err := retainedCollectionProjection(repository.UserActivities, schema)
		if err == nil || !strings.Contains(err.Error(), "taken_at") {
			t.Fatalf("retainedCollectionProjection() error = %v, want missing taken_at rejection", err)
		}
	})

	t.Run("order history canonical schema is accepted", func(t *testing.T) {
		t.Parallel()
		schema := retainedTestSchema(
			"seat_id",
			"ordered_at",
			"user_id",
			"menu_code",
			"is_member_seat",
			"__key__",
			"__error__",
			"__has_error__",
		)

		if _, err := retainedCollectionProjection(repository.OrderHistory, schema); err != nil {
			t.Fatalf("retainedCollectionProjection() error = %v", err)
		}
	})
}
