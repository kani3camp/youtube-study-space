package mybigquery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
)

func TestCurrentArchiveDeletionUsesBoundParameterStableJobAndReread(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const project = "demo-youtube-study-space-ci"
	const channel = "UCsynthetic0000000000001"
	const operation = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	deletes, reads := 0, 0
	lostACK := true
	var queryBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body any
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/tables/"):
			if !strings.HasSuffix(r.URL.Path, "/user-activity-history") {
				w.WriteHeader(404)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": 404, "message": "not found"}}))
				return
			}
			body = map[string]any{"schema": map[string]any{"fields": []any{map[string]any{"name": "user_id", "type": "STRING"}}}}
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/datasets/"):
			body = map[string]any{"location": "US"}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/jobs"):
			require.NoError(t, json.NewDecoder(r.Body).Decode(&queryBody))
			configuration, ok := queryBody["configuration"].(map[string]any)
			require.True(t, ok)
			config, ok := configuration["query"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "DELETE FROM `"+project+".firestore_export.user-activity-history` WHERE user_id = @channel", config["query"])
			require.NotContains(t, config["query"], channel)
			params, ok := config["queryParameters"].([]any)
			require.True(t, ok)
			parameter, ok := params[0].(map[string]any)
			require.True(t, ok)
			value, ok := parameter["parameterValue"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, channel, value["value"])
			deletes++
			body = queryBody
			queryBody["status"] = map[string]any{"state": "DONE"}
			if lostACK {
				lostACK = false
				w.WriteHeader(409)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": 409, "message": "job already exists"}}))
				return
			}
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/jobs/"):
			body = queryBody
		case (r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/queries")) || (r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/queries/")):
			if r.Method == http.MethodPost {
				reads++
			}
			body = map[string]any{"jobComplete": true, "totalRows": "1", "schema": map[string]any{"fields": []any{map[string]any{"name": "row_count", "type": "INTEGER"}}}, "rows": []any{map[string]any{"f": []any{map[string]any{"v": "0"}}}}}
		default:
			t.Errorf("unexpected synthetic SDK request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(body))
	}))
	defer server.Close()
	client, err := bigquery.NewClient(ctx, project, option.WithEndpoint(server.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	defer client.Close() //nolint:errcheck // Synthetic client.
	c := &BigqueryController{Client: client}
	require.NoError(t, c.ApplyChannelDeletion(ctx, channel, operation, false))
	require.Equal(t, 1, deletes)
	require.Equal(t, 1, reads)
	require.NoError(t, c.ApplyChannelDeletion(ctx, channel, operation, true))
	require.Equal(t, 1, deletes)
	require.Equal(t, 2, reads)
}
