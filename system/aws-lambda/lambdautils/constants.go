package lambdautils

const (
	OK = "ok"
)

type UserRPParallelRequest struct {
	ProcessIndex int      `json:"process_index"`
	UserIDs      []string `json:"user_ids"`
}
