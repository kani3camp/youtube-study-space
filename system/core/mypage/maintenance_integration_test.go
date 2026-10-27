//go:build integration

package mypage

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"

	"app.modules/internal/integrationtest"
)

func TestMaintenanceOperatorAndProtectedWrite(t *testing.T) {
	integrationtest.RequireFirestoreEmulator(t)
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, "demo-youtube-study-space-ci", option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ref := maintenanceRef(client)
	if _, err := ref.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	defer ref.Delete(ctx)
	for _, tt := range []struct{ enabled, changed bool }{{false, true}, {true, true}, {true, false}, {false, true}} {
		value, changed, err := SetMaintenance(ctx, client, tt.enabled)
		if err != nil || value.Enabled != tt.enabled || changed != tt.changed || value.UpdatedAt.IsZero() {
			t.Fatalf("set %v: %+v changed=%v err=%v", tt.enabled, value, changed, err)
		}
	}
	_, _, err = SetMaintenance(ctx, client, true)
	if err != nil {
		t.Fatal(err)
	}
	output := client.Collection("maintenance-test").Doc("protected")
	defer output.Delete(ctx)
	err = client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := transactionMaintenance(tx, client); err != nil {
			return err
		}
		return tx.Set(output, map[string]interface{}{"value": "written"})
	})
	if errorCode(err) != "MAINTENANCE_ACTIVE" {
		t.Fatalf("write error: %v", err)
	}
	doc, _ := output.Get(ctx)
	if doc != nil && doc.Exists() {
		t.Fatal("protected write committed")
	}
	_, _, err = SetMaintenance(ctx, client, false)
	if err != nil {
		t.Fatal(err)
	}
	err = client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		if err := transactionMaintenance(tx, client); err != nil {
			return err
		}
		return tx.Set(output, map[string]interface{}{"value": "written"})
	})
	if err != nil {
		t.Fatalf("normal write: %v", err)
	}
	doc, err = output.Get(ctx)
	if err != nil || doc.Data()["value"] != "written" {
		t.Fatalf("write missing: %v", err)
	}
	if _, err := ref.Set(ctx, map[string]interface{}{"maintenance-enabled": "bad", "updated-at": time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := SetMaintenance(ctx, client, false); err == nil {
		t.Fatal("malformed document accepted")
	}
}

func TestMaintenanceStopsWebAccountCommit(t *testing.T) {
	service, store, _, minter, now := authTestService(t)
	ctx := context.Background()
	_, _, err := SetMaintenance(ctx, store.Client, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = maintenanceRef(store.Client).Delete(ctx) })
	store.MaintenanceEnabled = true
	uid := "UCsynthetic0000000000001"
	account := store.Client.Collection("web-accounts").Doc(uid)
	if _, err := account.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = account.Delete(ctx) })
	id, confirmation := seedTransaction(t, service, "channel_verified", now)
	t.Cleanup(func() { _, _ = store.Client.Collection("oauth-transactions").Doc(id).Delete(ctx) })
	_, _, err = SetMaintenance(ctx, store.Client, true)
	if err != nil {
		t.Fatal(err)
	}
	token, err := service.Confirm(ctx, id, confirmation)
	if token != "" || errorCode(err) != "MAINTENANCE_ACTIVE" || minter.calls.Load() != 0 {
		t.Fatalf("token=%q err=%v mint calls=%d", token, err, minter.calls.Load())
	}
	doc, _ := account.Get(ctx)
	if doc != nil && doc.Exists() {
		t.Fatal("WebAccount committed during maintenance")
	}
	if _, err := store.ReadVerified(ctx, id, now); err != nil {
		t.Fatalf("OAuth consumption did not roll back: %v", err)
	}
}
