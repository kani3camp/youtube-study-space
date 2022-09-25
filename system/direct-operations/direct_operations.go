package direct_operations

import (
	"app.modules/core"
	"app.modules/core/utils"
	"cloud.google.com/go/firestore"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/api/option"
	"log"
	"math"
	"os"
)

func ExitAllUsersInRoom(ctx context.Context, clientOption option.ClientOption) {
	fmt.Println("全ユーザーを退室させます。よろしいですか？(yes / no)")
	var s string
	if _, err := fmt.Scanf("%s", &s); err != nil {
		panic(err)
		return
	}
	if s != "yes" {
		return
	}
	
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		panic(err)
		return
	}
	
	sys.MessageToOwner("direct op: ExitAllUsersInRoom")
	
	log.Println("全ユーザーを退室させます。")
	err = sys.ExitAllUsersInRoom(ctx)
	if err != nil {
		panic(err)
		return
	}
	log.Println("全ユーザーを退室させました。")
}

func ExitSpecificUser(ctx context.Context, userId string, clientOption option.ClientOption) {
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		panic(err)
		return
	}
	
	sys.MessageToOwner("direct op: ExitSpecificUser")
	
	sys.SetProcessedUser(userId, "**", false, false)
	outCommandDetails := core.CommandDetails{
		CommandType: core.Out,
	}
	
	err = sys.Out(outCommandDetails, ctx)
	if err != nil {
		panic(err)
		return
	}
}

func ExportUsersCollectionJson(ctx context.Context, clientOption option.ClientOption) {
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		panic(err)
		return
	}
	
	sys.MessageToOwner("direct op: ExportUsersCollectionJson")
	
	var allUsersTotalStudySecList []core.UserIdTotalStudySecSet
	err = sys.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var err error
		allUsersTotalStudySecList, err = sys.GetAllUsersTotalStudySecList(ctx)
		if err != nil {
			panic(err)
		}
		return nil
	})
	
	now := utils.JstNow()
	dateString := now.Format("2006-01-02_15-04-05")
	f, err := os.Create("./" + dateString + "_user-total-study-sec-list.json")
	if err != nil {
		panic(err)
		return
	}
	defer func() { _ = f.Close() }()
	
	jsonEnc := json.NewEncoder(f)
	//jsonEnc.SetIndent("", "\t")
	err = jsonEnc.Encode(allUsersTotalStudySecList)
	if err != nil {
		panic(err)
		return
	}
	log.Println("finished exporting json.")
}

func UpdateUsersRP(ctx context.Context, clientOption option.ClientOption) {
	sys, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		panic(err)
		return
	}
	
	sys.MessageToOwner("direct op: UpdateUsersRP")
	
	err, userIdsToProcessRP := sys.GetUserIdsToProcessRP(ctx)
	if err != nil {
		log.Println("failed to GetUserIdsToProcessRP", err)
		panic(err)
	}
	
	remainingUserIds, err := sys.UpdateUserRPBatch(ctx, userIdsToProcessRP, math.MaxInt)
	if err != nil {
		log.Println("failed to UpdateUserRPBatch", err)
		panic(err)
	}
	
	log.Println("remaining user ids:")
	log.Println(remainingUserIds)
}
