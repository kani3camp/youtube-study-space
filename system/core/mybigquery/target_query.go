package mybigquery

import (
	"errors"
	"fmt"
	"regexp"
)

var targetIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,127}$`)

// TargetDeletionSQL is a pure planner for synthetic deletion adapters. Values
// remain bound parameters; identifiers must come from a verified inventory.
// It neither constructs an SDK query nor authenticates or executes anything.
func TargetDeletionSQL(project, dataset, table, field string) (string, error) {
	if !targetIdentifier.MatchString(project) || !targetIdentifier.MatchString(dataset) || !targetIdentifier.MatchString(table) || (field != "user_id" && field != "author_channel_id") {
		return "", errors.New("invalid target deletion query")
	}
	return fmt.Sprintf("DELETE FROM `%s.%s.%s` WHERE %s = @channel", project, dataset, table, field), nil
}
