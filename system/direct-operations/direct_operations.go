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
	"os"
)

func ExitAllUsersInRoom(clientOption option.ClientOption, ctx context.Context) {
	fmt.Println("全ユーザーを退室させます。よろしいですか？(yes / no)")
	var s string
	_, _ = fmt.Scanf("%s", &s)
	if s != "yes" {
		return
	}
	
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		panic(err)
		return
	}
	
	_system.MessageToLiveChat(ctx, nil, "全ユーザーを退室させます。")
	err = _system.ExitAllUserInRoom(ctx)
	if err != nil {
		panic(err)
		return
	}
	_system.MessageToLiveChat(ctx, nil, "全ユーザーを退室させました。")
}

func ExitSpecificUser(userId string, clientOption option.ClientOption, ctx context.Context) {
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		panic(err)
		return
	}
	
	_system.SetProcessedUser(userId, "**", false, false)
	outCommandDetails := core.CommandDetails{
		CommandType: core.Out,
	}
	
	err = _system.Out(outCommandDetails, ctx)
	if err != nil {
		panic(err)
		return
	}
}

func ExportUsersCollectionJson(clientOption option.ClientOption, ctx context.Context) {
	_system, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		panic(err)
		return
	}
	
	var allUsersTotalStudySecList []core.UserIdTotalStudySecSet
	err = _system.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		var err error
		allUsersTotalStudySecList, err = _system.RetrieveAllUsersTotalStudySecList(ctx, tx)
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
