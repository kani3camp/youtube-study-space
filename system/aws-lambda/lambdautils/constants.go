package lambdautils

const (
	OK                           = "ok"
	ERROR                        = "error"
	InterruptionTimeLimitSeconds = 13 * 60 // 13分経って終わってなかったら次のlambdaを呼び出す。
)

// ProcessUserRPParallelRequestStruct 複数のファイルで使用するため、build時にundefinedとならないようにここで宣言。
type ProcessUserRPParallelRequestStruct struct {
	ProcessIndex int      `json:"process_index"`
	UserIds      []string `json:"user_ids"`
}
