package mypage

import "time"

type Availability string

const (
	Available   Availability = "available"
	Partial     Availability = "partial"
	Unavailable Availability = "unavailable"
)

const (
	SourceUnavailable     = "SOURCE_UNAVAILABLE"
	HistoryIncomplete     = "HISTORY_INCOMPLETE"
	HistoryLimitExceeded  = "HISTORY_LIMIT_EXCEEDED"
	DataInconsistent      = "DATA_INCONSISTENT"
	MetadataRefreshFailed = "METADATA_REFRESH_FAILED"
	MetadataTooOld        = "METADATA_TOO_OLD"
)

type Section[T any] struct {
	Availability Availability `json:"availability"`
	ReasonCode   *string      `json:"reasonCode"`
	Data         *T           `json:"data"`
}

type Metric struct {
	Availability Availability `json:"availability"`
	ReasonCode   *string      `json:"reasonCode"`
	WorkSec      *int64       `json:"workSec"`
}

type Current struct {
	State          string     `json:"state"`
	WorkName       *string    `json:"workName"`
	RoomType       *string    `json:"roomType"`
	SeatNumber     *int       `json:"seatNumber"`
	StateStartedAt *time.Time `json:"stateStartedAt"`
	ExpectedEndAt  *time.Time `json:"expectedEndAt"`
}

type Summary struct {
	Today    Metric `json:"today"`
	Week     Metric `json:"week"`
	Lifetime Metric `json:"lifetime"`
}

type Day struct {
	Date string `json:"date"`
	Metric
}

// RecentSection retains all seven dates even when every metric is unavailable.
type RecentSection struct {
	Availability Availability `json:"availability"`
	ReasonCode   *string      `json:"reasonCode"`
	Data         []Day        `json:"data"`
}

type Account struct {
	DisplayName string  `json:"displayName"`
	Handle      *string `json:"handle"`
	AvatarURL   *string `json:"avatarUrl"`
}

type Response struct {
	GeneratedAt time.Time        `json:"generatedAt"`
	Timezone    string           `json:"timezone"`
	Partial     bool             `json:"partial"`
	Current     Section[Current] `json:"current"`
	Summary     Section[Summary] `json:"summary"`
	Recent7Days RecentSection    `json:"recent7Days"`
	Account     Section[Account] `json:"account"`
}

func ptr[T any](v T) *T { return &v }

func valueMetric(seconds int64) Metric {
	return Metric{Availability: Available, WorkSec: ptr(seconds)}
}

func missingMetric(reason string) Metric {
	return Metric{Availability: Unavailable, ReasonCode: ptr(reason)}
}

func availableSection[T any](v T) Section[T] {
	return Section[T]{Availability: Available, Data: &v}
}

func missingSection[T any](reason string) Section[T] {
	return Section[T]{Availability: Unavailable, ReasonCode: &reason}
}
