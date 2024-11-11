package lambdautils

const (
	OK                    = "ok"
	InterruptTimeLimitSec = 13 * 60 // If a lambda function does not terminate after 13 minutes, call the next lambda function.
)

type UserRPParallelRequest struct {
	ProcessIndex int      `json:"process_index"`
	UserIds      []string `json:"user_ids"`
}
