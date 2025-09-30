package lambdautils

const (
	OK = "ok"
)

type UserRPParallelRequest struct {
	ProcessIndex int      `json:"process_index"`
	UserIds      []string `json:"user_ids"`
}
