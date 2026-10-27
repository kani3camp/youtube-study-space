package mypage

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	MaintenanceNormal          = "NORMAL"
	MaintenanceActive          = "ACTIVE"
	MaintenanceMessageTemplate = "MYPAGE_MAINTENANCE_V1"
)

type MaintenanceControl struct {
	Enabled   bool      `firestore:"maintenance-enabled"`
	UpdatedAt time.Time `firestore:"updated-at"`
}

type MaintenanceStatus struct {
	State           string    `json:"state"`
	UpdatedAt       time.Time `json:"updatedAt"`
	MessageTemplate string    `json:"messageTemplate"`
}

type MaintenanceReader interface {
	Check(context.Context) error
	PublicStatus(context.Context) (MaintenanceStatus, error)
}

type MaintenanceGate struct {
	Client      *firestore.Client
	readControl func(context.Context) (MaintenanceControl, error)
}

func maintenanceRef(client *firestore.Client) *firestore.DocumentRef {
	return client.Collection("config").Doc("mypage")
}

func decodeMaintenanceDocument(doc *firestore.DocumentSnapshot) (MaintenanceControl, error) {
	if doc == nil || !doc.Exists() {
		return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	data := doc.Data()
	enabled, ok := data["maintenance-enabled"].(bool)
	updated, valid := data["updated-at"].(time.Time)
	if !ok || !valid || updated.IsZero() {
		return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	return MaintenanceControl{Enabled: enabled, UpdatedAt: updated}, nil
}

func (g *MaintenanceGate) read(ctx context.Context) (MaintenanceControl, error) {
	if g != nil && g.readControl != nil {
		return g.readControl(ctx)
	}
	if g == nil || g.Client == nil {
		return MaintenanceControl{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	doc, err := maintenanceRef(g.Client).Get(ctx)
	if err != nil {
		return MaintenanceControl{}, fmt.Errorf("read maintenance control: %w: %w", apiError("TEMPORARY_UNAVAILABLE"), err)
	}
	return decodeMaintenanceDocument(doc)
}

func (g *MaintenanceGate) Check(ctx context.Context) error {
	value, err := g.read(ctx)
	if err != nil {
		return err
	}
	if value.Enabled {
		return apiError("MAINTENANCE_ACTIVE")
	}
	return nil
}

func (g *MaintenanceGate) PublicStatus(ctx context.Context) (MaintenanceStatus, error) {
	value, err := g.read(ctx)
	if err != nil {
		return MaintenanceStatus{}, err
	}
	state := MaintenanceNormal
	if value.Enabled {
		state = MaintenanceActive
	}
	return MaintenanceStatus{State: state, UpdatedAt: value.UpdatedAt, MessageTemplate: MaintenanceMessageTemplate}, nil
}

// The control is read in the same transaction as a protected write, so a
// concurrent activation causes Firestore to retry against the new value.
func transactionMaintenance(tx *firestore.Transaction, client *firestore.Client) error {
	if client == nil {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	doc, err := tx.Get(maintenanceRef(client))
	if err != nil {
		return fmt.Errorf("read maintenance control in transaction: %w: %w", apiError("TEMPORARY_UNAVAILABLE"), err)
	}
	value, err := decodeMaintenanceDocument(doc)
	if err != nil {
		return err
	}
	if value.Enabled {
		return apiError("MAINTENANCE_ACTIVE")
	}
	return nil
}

// SetMaintenance creates a missing control document or changes it atomically.
// Existing malformed documents fail closed and require operator investigation.
func SetMaintenance(ctx context.Context, client *firestore.Client, enabled bool) (MaintenanceControl, bool, error) {
	if client == nil {
		return MaintenanceControl{}, false, apiError("TEMPORARY_UNAVAILABLE")
	}
	ref := maintenanceRef(client)
	changed := false
	err := client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		changed = false
		doc, err := tx.Get(ref)
		if err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("read maintenance control: %w", err)
		}
		if err == nil {
			previous, err := decodeMaintenanceDocument(doc)
			if err != nil {
				return err
			}
			if previous.Enabled == enabled {
				return nil
			}
		}
		changed = true
		return tx.Set(ref, map[string]interface{}{"maintenance-enabled": enabled, "updated-at": firestore.ServerTimestamp})
	})
	if err != nil {
		return MaintenanceControl{}, false, fmt.Errorf("set maintenance control: %w", err)
	}
	doc, err := ref.Get(ctx)
	if err != nil {
		return MaintenanceControl{}, false, fmt.Errorf("read back maintenance control: %w", err)
	}
	value, err := decodeMaintenanceDocument(doc)
	if err != nil {
		return MaintenanceControl{}, false, err
	}
	if value.Enabled != enabled {
		return MaintenanceControl{}, false, fmt.Errorf("maintenance readback changed")
	}
	return value, changed, nil
}
