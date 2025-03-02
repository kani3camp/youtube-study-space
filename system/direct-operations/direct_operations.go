package direct_operations

import (
	"app.modules/core/workspaceapp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"

	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
)

func ExitAllUsersInRoom(ctx context.Context, clientOption option.ClientOption) {
	fmt.Println("全ルームの全ユーザーを退室させます。よろしいですか？(yes / no)")
	var s string
	if _, err := fmt.Scanln(&s); err != nil {
		panic(err)
	}
	if s != "yes" {
		return
	}

	sys, err := workspaceapp.NewSystem(ctx, true, clientOption)
	if err != nil {
		panic(err)
	}

	sys.MessageToOwner(ctx, "direct op: ExitAllUsersInRoom")

	slog.Info("全ルームの全ユーザーを退室させます。")
	if err := sys.ExitAllUsersInRoom(ctx, true); err != nil {
		panic(err)
	}
	if err := sys.ExitAllUsersInRoom(ctx, false); err != nil {
		panic(err)
	}

	slog.Info("全ルームの全ユーザーを退室させました。")
}

func ExitSpecificUser(ctx context.Context, userId string, clientOption option.ClientOption) {
	sys, err := workspaceapp.NewSystem(ctx, true, clientOption)
	if err != nil {
		panic(err)
	}

	sys.MessageToOwner(ctx, "direct op: ExitSpecificUser")

	sys.SetProcessedUser(userId, "**", "**", false, false, true)
	outCommandDetails := &utils.CommandDetails{
		CommandType: utils.Out,
	}

	if err = sys.Out(outCommandDetails, ctx); err != nil {
		panic(err)
	}
}

func ExportUsersCollectionJson(ctx context.Context, clientOption option.ClientOption) {
	sys, err := workspaceapp.NewSystem(ctx, true, clientOption)
	if err != nil {
		panic(err)
	}

	sys.MessageToOwner(ctx, "direct op: ExportUsersCollectionJson")

	var allUsersTotalStudySecList []utils.UserIdTotalStudySecSet
	txErr := sys.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var err error
		allUsersTotalStudySecList, err = sys.GetAllUsersTotalStudySecList(ctx)
		if err != nil {
			panic(err)
		}
		return nil
	})
	if txErr != nil {
		panic(txErr)
	}

	now := utils.JstNow()
	dateString := now.Format("2006-01-02_15-04-05")
	f, err := os.Create("./" + dateString + "_user-total-study-sec-list.json")
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()

	jsonEnc := json.NewEncoder(f)
	//jsonEnc.SetIndent("", "\t")
	if err := jsonEnc.Encode(allUsersTotalStudySecList); err != nil {
		panic(err)
	}
	slog.Info("finished exporting json.")
}

func UpdateUsersRP(ctx context.Context, clientOption option.ClientOption) {
	sys, err := workspaceapp.NewSystem(ctx, true, clientOption)
	if err != nil {
		panic(err)
	}

	sys.MessageToOwner(ctx, "direct op: UpdateUsersRP")

	userIdsToProcessRP, err := sys.GetUserIdsToProcessRP(ctx)
	if err != nil {
		slog.Error("error in GetUserIdsToProcessRP.", "err", err)
		panic(err)
	}

	remainingUserIds := sys.UpdateUserRPBatch(ctx, userIdsToProcessRP, math.MaxInt)

	slog.Info("", "remaining user ids", remainingUserIds)
}
