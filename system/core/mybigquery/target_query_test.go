package mybigquery

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTargetDeletionPlannerBindsValueAndRejectsIdentifiersOutsideInventoryContract(t *testing.T) {
	for _, field := range []string{"user_id", "author_channel_id"} {
		sql, err := TargetDeletionSQL("demo-youtube-study-space-ci", DatasetName, TemporaryTableName, field)
		require.NoError(t, err)
		require.Contains(t, sql, "WHERE "+field+" = @channel")
		require.Equal(t, 1, strings.Count(sql, "DELETE"))
		require.NotContains(t, sql, "TRUNCATE")
	}
	for _, values := range [][4]string{
		{"p` WHERE TRUE; --", DatasetName, TemporaryTableName, "user_id"},
		{"project", "dataset.name", "tmp", "user_id"},
		{"project", "dataset", "tmp`", "user_id"},
		{"project", "dataset", "tmp", "user_id OR TRUE"},
		{"project", "dataset", "tmp", ""},
	} {
		sql, err := TargetDeletionSQL(values[0], values[1], values[2], values[3])
		require.Error(t, err)
		require.Empty(t, sql)
	}
}
