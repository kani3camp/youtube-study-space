package system

import (
	"app.modules/system/customerror"
	"app.modules/system/myfirestore"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io/ioutil"
	"log"
	"reflect"
	"strconv"
	"time"
)

const (
	ActionFirestore = "action"
	OldRoomLayoutFirestore = "old-room-layout"
	NewRoomLayoutFirestore = "new-room-layout"
	DateFirestore = "date"
	
	UpdateRoomLayoutAction = "update-room-layout"
)

// UpdateRoomLayout: ルームレイアウトを更新。ルームレイアウトが存在しなければ、新規作成
func (s *System) UpdateRoomLayout(filePath string, ctx context.Context) {
	rawData, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	var roomLayout myfirestore.RoomLayoutDoc
	err = json.Unmarshal(rawData, &roomLayout)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	customErr := s.CheckRoomLayoutData(roomLayout, ctx)
	if customErr.Body != nil {
		fmt.Println(customErr.Body.Error())
		return
	}
	fmt.Println("Valid layout file.")
	err = s.SaveRoomLayout(roomLayout, ctx)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
}

// CheckRoomLayoutData: ルーム作成時の roomLayoutData.Version は 1
func (s *System) CheckRoomLayoutData(roomLayoutData myfirestore.RoomLayoutDoc, ctx context.Context) customerror.CustomError {
	fmt.Println("CheckRoomLayoutData()")
	var idList []int
	var partitionShapeTypeList []string
	
	// default-roomが存在するか
	isExistRoom, err := s.IsExistRoomLayout(ctx)
	if err != nil {
		return customerror.Unknown.Wrap(err)
	}
	if roomLayoutData.Version > 1 && (!isExistRoom) {
		return customerror.InvalidRoomLayout.New("default room layout doesn't exist")
	} else if currentVersion, _ := s.CurrentDefaultRoomLayoutVersion(ctx); roomLayoutData.Version != 1+currentVersion {
		return customerror.InvalidRoomLayout.New("please specify a incremented version. latest version is " + strconv.Itoa(currentVersion))
	} else if roomLayoutData.FontSizeRatio == 0.0 {
		return customerror.InvalidRoomLayout.New("please specify a valid font size ratio")
	} else if roomLayoutData.RoomShape.Height == 0 || roomLayoutData.RoomShape.Width == 0 {
		return customerror.InvalidRoomLayout.New("please specify the room-shape correctly")
	}
	
	if len(roomLayoutData.PartitionShapes) > 0 {
		// PartitionのShapeTypeの重複がないか
		for _, p := range roomLayoutData.PartitionShapes {
			if p.Name == "" || p.Width == 0 || p.Height == 0 {
				return customerror.InvalidRoomLayout.New("please specify partition shapes correctly")
			} // ここから正常にifを抜けることがある
			for _, other := range partitionShapeTypeList {
				if other == p.Name {
					return customerror.InvalidRoomLayout.New("some partition shape types are duplicated")
				}
			}
			partitionShapeTypeList = append(partitionShapeTypeList, p.Name)
		}
	}
	if len(roomLayoutData.Seats) == 0 {
		return customerror.InvalidRoomLayout.New("please specify at least one seat")
	}
	// SeatのIdの重複がないか
	for _, s := range roomLayoutData.Seats {
		for _, other := range idList {
			if other == s.Id {
				return customerror.InvalidRoomLayout.New("some seat ids are duplicated")
			}
		}
		idList = append(idList, s.Id)
	}
	// 仕切り
	for _, p := range roomLayoutData.Partitions {
		if p.ShapeType == "" {
			return customerror.InvalidRoomLayout.New("please specify valid shape-type to all shapes")
		}
		// 仕切りのShapeTypeに有効なものが指定されているか
		isContained := false
		for _, other := range partitionShapeTypeList {
			if other == p.ShapeType {
				isContained = true
			}
		}
		if !isContained {
			return customerror.InvalidRoomLayout.New("please specify valid shape type, at partition id = " + strconv.Itoa(p.Id))
		}
	}
	return customerror.NewNilCustomError()
}

func (s *System) IsExistRoomLayout(ctx context.Context) (bool, error) {
	_, err := s.FirestoreController.RetrieveDefaultRoomLayout(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *System) CurrentDefaultRoomLayoutVersion(ctx context.Context) (int, error) {
	roomLayout, err := s.FirestoreController.RetrieveDefaultRoomLayout(ctx)
	if err != nil {
		return 0, err
	}
	return roomLayout.Version, nil
}

func (s *System) SaveRoomLayout(roomLayout myfirestore.RoomLayoutDoc, ctx context.Context) error {
	fmt.Println("SaveRoomLayout()")
	
	// 履歴を保存
	var oldRoomLayout myfirestore.RoomLayoutDoc
	var err error
	if roomLayout.Version == 1 { // 最初のアップロードだと既存のレイアウトデータは存在しない
		oldRoomLayout = myfirestore.RoomLayoutDoc{}
	} else {
		oldRoomLayout, err = s.FirestoreController.RetrieveDefaultRoomLayout(ctx)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	_ = s.FirestoreController.AddRoomLayoutHistory(map[string]interface{}{
		ActionFirestore: UpdateRoomLayoutAction,
		OldRoomLayoutFirestore: oldRoomLayout,
		NewRoomLayoutFirestore: roomLayout,
		DateFirestore:            time.Now(),
	}, ctx)
	
	// 前後で座席に変更があった場合、現在そのルームにいる人を強制的に退室させる
	// 現在の座席idリスト
	var oldSeatIds []int
	for _, oldSeat := range oldRoomLayout.Seats {
		oldSeatIds = append(oldSeatIds, oldSeat.Id)
	}
	// 新レイアウトの座席idリスト
	var newSeatIds []int
	for _, newSeat := range roomLayout.Seats {
		newSeatIds = append(newSeatIds, newSeat.Id)
	}
	if !reflect.DeepEqual(oldSeatIds, newSeatIds) {
		fmt.Println("oldSeatIds != newSeatIds. so all users in the room will forcibly be left")
		err := s.ExitAllUserDefaultRoom(ctx)
		if err != nil {
			return err
		}
	}
	// 保存
	err = s.FirestoreController.SaveRoomLayout(roomLayout, ctx)
	if err != nil {
		return err
	}
	return nil
}







