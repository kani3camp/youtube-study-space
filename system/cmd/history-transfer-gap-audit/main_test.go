package main

import (
	"reflect"
	"testing"
	"time"

	"app.modules/core/repository"
)

func TestParseFromExplicitDate(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseFrom("2026-06-23", time.Now(), location)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 6, 23, 0, 0, 0, 0, location)
	if !got.Equal(want) {
		t.Fatalf("parseFrom() = %s, want %s", got, want)
	}
}

func TestTargetCollection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "user activities",
			path: "2026-08-13T15:00:06_83201/all_namespaces/kind_user-activities/output-0",
			want: repository.UserActivities,
		},
		{
			name: "order history",
			path: "2026-08-13T15:00:06_83201/all_namespaces/kind_order-history/output-0",
			want: repository.OrderHistory,
		},
		{
			name: "unrelated collection",
			path: "2026-08-13T15:00:06_83201/all_namespaces/kind_users/output-0",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := targetCollection(tt.path); got != tt.want {
				t.Fatalf("targetCollection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSortedDailyCounts(t *testing.T) {
	t.Parallel()
	got := sortedDailyCounts(map[string]int64{
		"2026-08-13": 2,
		"2026-08-11": 3,
		"2026-08-12": 1,
	})
	want := []dailyCount{
		{Date: "2026-08-11", Rows: 3},
		{Date: "2026-08-12", Rows: 1},
		{Date: "2026-08-13", Rows: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedDailyCounts() = %#v, want %#v", got, want)
	}
}
