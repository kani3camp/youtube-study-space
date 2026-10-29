package mybigquery

import (
	"errors"
	"fmt"
	"regexp"
)

var targetIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,127}$`)

// TargetDeletionSQLは、合成削除アダプター用の純粋な計画処理である。
// 値はバインドパラメーターのまま維持し、識別子は検証済み一覧から取得する。
// SDKクエリの構築・認証・実行は行わない。
func TargetDeletionSQL(project, dataset, table, field string) (string, error) {
	if !targetIdentifier.MatchString(project) || !targetIdentifier.MatchString(dataset) || !targetIdentifier.MatchString(table) || (field != "user_id" && field != "author_channel_id") {
		return "", errors.New("invalid target deletion query")
	}
	return fmt.Sprintf("DELETE FROM `%s.%s.%s` WHERE %s = @channel", project, dataset, table, field), nil
}
