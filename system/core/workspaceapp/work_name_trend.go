package workspaceapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"app.modules/core/repository"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"github.com/openai/openai-go/v2" // imported as openai
	"github.com/openai/openai-go/v2/option"
	"github.com/openai/openai-go/v2/responses"
	"github.com/openai/openai-go/v2/shared"
)

func (app *WorkspaceApp) UpdateWorkNameTrend(ctx context.Context, apiKey string) error {
	slog.Info(utils.NameOf(app.UpdateWorkNameTrend))

	var workNames []string
	generalSeats, err := app.Repository.ReadActiveWorkNameSeats(ctx, true)
	if err != nil {
		return fmt.Errorf("in ReadActiveWorkNameSeats(): %w", err)
	}
	memberSeats, err := app.Repository.ReadActiveWorkNameSeats(ctx, false)
	if err != nil {
		return fmt.Errorf("in ReadActiveWorkNameSeats(): %w", err)
	}

	for _, seat := range generalSeats {
		workNames = append(workNames, seat.WorkName)
	}
	for _, seat := range memberSeats {
		workNames = append(workNames, seat.WorkName)
	}

	// AIで作業内容のトレンドを導く
	client := openai.NewClient(option.WithAPIKey(apiKey))

	userInput := strings.Join(workNames, "\n")
	slog.Info("userInput", "value", userInput)

	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Model: shared.ResponsesModel("gpt-5-nano"),
		Reasoning: shared.ReasoningParam{
			Effort:  shared.ReasoningEffortMedium,
			Summary: shared.ReasoningSummaryAuto,
		},
		Store: openai.Bool(true),
		Instructions: openai.String(`作業内容の一覧を改行区切りで入力します。
作業内容のトレンドのジャンルをランキング形式で上位5個を列挙してください。
ランキングでは、そのジャンル名、代表的な作業項目（例）を含めるようにしてください。
ただし、トレンドになっているとは言い難いものは、無理にランキングする必要はありません。
絵文字だけの作業内容もありますが、絵文字から意味を推測してください。
その他、作業内容の意味がわかりづらいものは集計では無視してください。
`),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(userInput),
		},
		Text: responses.ResponseTextConfigParam{
			Format: responses.ResponseFormatTextConfigUnionParam{
				OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
					Name: "ranking_top5",
					Schema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"rankings": map[string]interface{}{
								"type":        "array",
								"description": "ランキングのリスト（最大５件）",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"rank": map[string]interface{}{
											"type":        "integer",
											"description": "順位（1〜5）",
											"minimum":     1,
											"maximum":     5,
										},
										"genre": map[string]interface{}{
											"type":        "string",
											"description": "ジャンル名",
										},
										"count": map[string]interface{}{
											"type":        "integer",
											"description": "対象作業項目のカウント数",
											"minimum":     1,
										},
										"examples": map[string]interface{}{
											"type":        "array",
											"description": "代表的な作業項目抽出例",
											"minItems":    1,
											"maxItems":    5,
											"items": map[string]interface{}{
												"type":        "string",
												"description": "作業項目名",
											},
										},
									},
									"required":             []string{"rank", "genre", "count", "examples"},
									"additionalProperties": false,
								},
								"minItems": 0,
								"maxItems": 5,
							},
						},
						"required":             []string{"rankings"},
						"additionalProperties": false,
					},
					Strict: openai.Bool(true),
				},
			},
			Verbosity: responses.ResponseTextConfigVerbosityMedium,
		},
	})
	if err != nil {
		return fmt.Errorf("in client.Responses.New(): %w", err)
	}

	slog.Info("resp.OutputText()", "value", resp.OutputText())

	type workNameTrendResponse struct {
		Rankings []repository.WorkNameTrendRanking `json:"rankings"`
	}
	var result workNameTrendResponse
	err = json.Unmarshal([]byte(resp.OutputText()), &result)
	if err != nil {
		return fmt.Errorf("in json.Unmarshal(): %w", err)
	}

	// DBに保存
	workNameTrend := repository.WorkNameTrendDoc{
		Ranking:  result.Rankings,
		RankedAt: time.Now(),
	}
	txErr := app.Repository.FirestoreClient().RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return app.Repository.UpdateWorkNameTrend(ctx, tx, workNameTrend)
	})
	if txErr != nil {
		return fmt.Errorf("in FirestoreClient().RunTransaction(): %w", txErr)
	}

	slog.Info(utils.NameOf(app.UpdateWorkNameTrend) + " finished")

	return nil
}
