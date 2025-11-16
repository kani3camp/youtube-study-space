package repository

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	mongoClient *mongo.Client
	once        sync.Once
)

// NewMongoClient initializes and returns a singleton MongoDB client.
// It reads the connection string from the MONGODB_URI environment variable.
func NewMongoClient() (*mongo.Client, error) {
	once.Do(func() {
		uri := os.Getenv("MONGODB_URI")
		if uri == "" {
			log.Fatal("You must set your 'MONGODB_URI' environmental variable.")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			log.Fatalf("failed to connect to mongo: %v", err)
		}

		mongoClient = client
	})

	return mongoClient, nil
}

type MongoController struct {
	Client *mongo.Client
	DB     *mongo.Database
}

func NewMongoController() (*MongoController, error) {
	client, err := NewMongoClient()
	if err != nil {
		return nil, err
	}
	db := client.Database("study_space_db")
	return &MongoController{
		Client: client,
		DB:     db,
	}, nil
}

func (c *MongoController) RunTransaction(ctx context.Context, f func(ctx context.Context) error) error {
	session, err := c.Client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, f(sessCtx)
	})
	return err
}

func (c *MongoController) DeleteDocRef(ctx context.Context, ref *firestore.DocumentRef) error {
	collectionName := ref.Parent.ID
	docID := ref.ID

	collection := c.DB.Collection(collectionName)
	filter := bson.M{"_id": docID}

	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete document %s/%s: %w", collectionName, docID, err)
	}
	return nil
}


func (c *MongoController) ReadCredentialsConfig(ctx context.Context) (CredentialsConfigDoc, error) {
	var config CredentialsConfigDoc
	err := c.DB.Collection("credentials").FindOne(ctx, nil).Decode(&config)
	return config, err
}

func (c *MongoController) ReadSystemConstantsConfig(ctx context.Context) (ConstantsConfigDoc, error) {
	var config ConstantsConfigDoc
	err := c.DB.Collection("constants").FindOne(ctx, nil).Decode(&config)
	return config, err
}

func (c *MongoController) ReadLiveChatId(ctx context.Context) (string, error) {
	var result struct {
		Value string `bson:"value"`
	}
	err := c.DB.Collection("configs").FindOne(ctx, bson.M{"_id": "liveChatId"}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil // Not found is not an error, return empty string
		}
		return "", fmt.Errorf("failed to read live chat ID: %w", err)
	}
	return result.Value, nil
}

func (c *MongoController) ReadNextPageToken(ctx context.Context) (string, error) {
	var result struct {
		Value string `bson:"value"`
	}
	err := c.DB.Collection("configs").FindOne(ctx, bson.M{"_id": "nextPageToken"}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil // Not found is not an error, return empty string
		}
		return "", fmt.Errorf("failed to read next page token: %w", err)
	}
	return result.Value, nil
}

func (c *MongoController) UpdateNextPageToken(ctx context.Context, nextPageToken string) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "nextPageToken"}
	update := bson.M{"$set": bson.M{"value": nextPageToken}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update next page token: %w", err)
	}
	return nil
}

func (c *MongoController) ReadGeneralSeats(ctx context.Context) ([]SeatDoc, error) {
	collection := c.DB.Collection("general_seats")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to read general seats: %w", err)
	}
	defer cursor.Close(ctx)

	var seats []SeatDoc
	if err = cursor.All(ctx, &seats); err != nil {
		return nil, fmt.Errorf("failed to decode general seats: %w", err)
	}
	return seats, nil
}

func (c *MongoController) ReadMemberSeats(ctx context.Context) ([]SeatDoc, error) {
	collection := c.DB.Collection("member_seats")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to read member seats: %w", err)
	}
	defer cursor.Close(ctx)

	var seats []SeatDoc
	if err = cursor.All(ctx, &seats); err != nil {
		return nil, fmt.Errorf("failed to decode member seats: %w", err)
	}
	return seats, nil
}

func (c *MongoController) ReadSeatsExpiredUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error) {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"until": bson.M{"$lte": thresholdTime}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to read expired seats: %w", err)
	}
	defer cursor.Close(ctx)

	var seats []SeatDoc
	if err = cursor.All(ctx, &seats); err != nil {
		return nil, fmt.Errorf("failed to decode expired seats: %w", err)
	}
	return seats, nil
}

func (c *MongoController) ReadSeatsExpiredBreakUntil(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatDoc, error) {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"break_until": bson.M{"$lte": thresholdTime}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to read expired break seats: %w", err)
	}
	defer cursor.Close(ctx)

	var seats []SeatDoc
	if err = cursor.All(ctx, &seats); err != nil {
		return nil, fmt.Errorf("failed to decode expired break seats: %w", err)
	}
	return seats, nil
}

func getSeatCollectionName(isMemberSeat bool) string {
	if isMemberSeat {
		return "member_seats"
	}
	return "general_seats"
}

func (c *MongoController) ReadSeat(ctx context.Context, seatId int, isMemberSeat bool) (SeatDoc, error) {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"_id": seatId}

	var seat SeatDoc
	err := collection.FindOne(ctx, filter).Decode(&seat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return SeatDoc{}, nil // Not found is not an error
		}
		return SeatDoc{}, fmt.Errorf("failed to read seat %d: %w", seatId, err)
	}
	return seat, nil
}

func (c *MongoController) ReadSeatWithUserId(ctx context.Context, userId string, isMemberSeat bool) (SeatDoc, error) {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"user_id": userId}

	var seat SeatDoc
	err := collection.FindOne(ctx, filter).Decode(&seat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return SeatDoc{}, nil // Not found is not an error
		}
		return SeatDoc{}, fmt.Errorf("failed to read seat with user ID %s: %w", userId, err)
	}
	return seat, nil
}

func (c *MongoController) ReadActiveWorkNameSeats(ctx context.Context, isMemberSeat bool) ([]SeatDoc, error) {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"work_name": bson.M{"$ne": ""}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to read active work name seats: %w", err)
	}
	defer cursor.Close(ctx)

	var seats []SeatDoc
	if err = cursor.All(ctx, &seats); err != nil {
		return nil, fmt.Errorf("failed to decode active work name seats: %w", err)
	}
	return seats, nil
}

func (c *MongoController) CreateSeat(ctx context.Context, seat SeatDoc, isMemberSeat bool) error {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)

	_, err := collection.InsertOne(ctx, seat)
	if err != nil {
		return fmt.Errorf("failed to create seat %d: %w", seat.ID, err)
	}
	return nil
}

func (c *MongoController) UpdateSeat(ctx context.Context, seat SeatDoc, isMemberSeat bool) error {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"_id": seat.ID}

	_, err := collection.ReplaceOne(ctx, filter, seat)
	if err != nil {
		return fmt.Errorf("failed to update seat %d: %w", seat.ID, err)
	}
	return nil
}

func (c *MongoController) DeleteSeat(ctx context.Context, seatId int, isMemberSeat bool) error {
	collectionName := getSeatCollectionName(isMemberSeat)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"_id": seatId}

	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete seat %d: %w", seatId, err)
	}
	return nil
}

func (c *MongoController) ReadUser(ctx context.Context, userId string) (UserDoc, error) {
	var user UserDoc
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	err := collection.FindOne(ctx, filter).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return UserDoc{}, nil // Not found is not an error
		}
		return UserDoc{}, err
	}
	return user, nil
}

func (c *MongoController) CreateUser(ctx context.Context, userId string, userData UserDoc) error {
	collection := c.DB.Collection("users")
	_, err := collection.InsertOne(ctx, userData)
	return err
}

func (c *MongoController) UpdateUserLastEnteredDate(ctx context.Context, userId string, enteredDate time.Time) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"last_entered": enteredDate}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserLastExitedDate(ctx context.Context, userId string, exitedDate time.Time) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"last_exited": exitedDate}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserRankVisible(ctx context.Context, userId string, rankVisible bool) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"rank_visible": rankVisible}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserDefaultStudyMin(ctx context.Context, userId string, defaultStudyMin int) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"default_study_min": defaultStudyMin}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserFavoriteColor(ctx context.Context, userId string, colorCode string) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"favorite_color": colorCode}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserTotalTime(ctx context.Context, userId string, newTotalTimeSec int, newDailyTotalTimeSec int) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{
		"total_study_sec":      newTotalTimeSec,
		"daily_total_study_sec": newDailyTotalTimeSec,
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserRankPoint(ctx context.Context, userId string, rp int) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"rank_point": rp}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserLastRPProcessed(ctx context.Context, userId string, date time.Time) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"last_rp_processed": date}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserRPAndLastPenaltyImposedDays(ctx context.Context, userId string, newRP int, newLastPenaltyImposedDays int) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{
		"rank_point":              newRP,
		"last_penalty_imposed_days": newLastPenaltyImposedDays,
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserIsContinuousActiveAndCurrentActivityStateStarted(ctx context.Context, userId string, isContinuousActive bool, currentActivityStateStarted time.Time) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{
		"is_continuous_active":        isContinuousActive,
		"current_activity_state_started": currentActivityStateStarted,
	}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateUserLastPenaltyImposedDays(ctx context.Context, userId string, lastPenaltyImposedDays int) error {
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"last_penalty_imposed_days": lastPenaltyImposedDays}}
	_, err := collection.UpdateOne(ctx, filter, update)
	return err
}

func (c *MongoController) UpdateLiveChatId(ctx context.Context, liveChatId string) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "liveChatId"}
	update := bson.M{"$set": bson.M{"value": liveChatId}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update live chat ID: %w", err)
	}
	return nil
}

func (c *MongoController) CreateLiveChatHistoryDoc(ctx context.Context, liveChatHistoryDoc LiveChatHistoryDoc) error {
	collection := c.DB.Collection("live_chat_history")
	_, err := collection.InsertOne(ctx, liveChatHistoryDoc)
	if err != nil {
		return fmt.Errorf("failed to create live chat history document: %w", err)
	}
	return nil
}

func (c *MongoController) Get500LiveChatHistoryDocIdsBeforeDate(ctx context.Context, date time.Time) ([]LiveChatHistoryDoc, error) {
	collection := c.DB.Collection("live_chat_history")
	filter := bson.M{"created_at": bson.M{"$lt": date}}
	opts := options.Find().SetSort(bson.M{"created_at": 1}).SetLimit(500)

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get live chat history documents: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []LiveChatHistoryDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode live chat history documents: %w", err)
	}
	return docs, nil
}

func (c *MongoController) CreateUserActivityDoc(ctx context.Context, activity UserActivityDoc) error {
	collection := c.DB.Collection("user_activities")
	_, err := collection.InsertOne(ctx, activity)
	if err != nil {
		return fmt.Errorf("failed to create user activity document: %w", err)
	}
	return nil
}

func (c *MongoController) Get500UserActivityDocIdsBeforeDate(ctx context.Context, date time.Time) ([]UserActivityDoc, error) {
	collection := c.DB.Collection("user_activities")
	filter := bson.M{"created_at": bson.M{"$lt": date}}
	opts := options.Find().SetSort(bson.M{"created_at": 1}).SetLimit(500)

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get user activity documents: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []UserActivityDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode user activity documents: %w", err)
	}
	return docs, nil
}

func (c *MongoController) GetAllUserActivityDocIdsAfterDate(ctx context.Context, date time.Time) ([]UserActivityDoc, error) {
	collection := c.DB.Collection("user_activities")
	filter := bson.M{"created_at": bson.M{"$gt": date}}
	opts := options.Find().SetSort(bson.M{"created_at": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get all user activity documents after date: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []UserActivityDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode user activity documents: %w", err)
	}
	return docs, nil
}

func (c *MongoController) Get500OrderHistoryDocIdsBeforeDate(ctx context.Context, date time.Time) ([]OrderHistoryDoc, error) {
	collection := c.DB.Collection("order_history")
	filter := bson.M{"created_at": bson.M{"$lt": date}}
	opts := options.Find().SetSort(bson.M{"created_at": 1}).SetLimit(500)

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get order history documents: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []OrderHistoryDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode order history documents: %w", err)
	}
	return docs, nil
}

func (c *MongoController) GetEnterRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context, date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	collection := c.DB.Collection("user_activities")
	filter := bson.M{
		"created_at":   bson.M{"$gt": date},
		"user_id":      userId,
		"seat_id":      seatId,
		"is_member_seat": isMemberSeat,
		"activity_type": "enter_room",
	}
	opts := options.Find().SetSort(bson.M{"created_at": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get enter room user activity documents: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []UserActivityDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode enter room user activity documents: %w", err)
	}
	return docs, nil
}

func (c *MongoController) GetExitRoomUserActivityDocIdsAfterDateForUserAndSeat(ctx context.Context, date time.Time, userId string, seatId int, isMemberSeat bool) ([]UserActivityDoc, error) {
	collection := c.DB.Collection("user_activities")
	filter := bson.M{
		"created_at":   bson.M{"$gt": date},
		"user_id":      userId,
		"seat_id":      seatId,
		"is_member_seat": isMemberSeat,
		"activity_type": "exit_room",
	}
	opts := options.Find().SetSort(bson.M{"created_at": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get exit room user activity documents: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []UserActivityDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode exit room user activity documents: %w", err)
	}
	return docs, nil
}

func (c *MongoController) GetUsersActiveAfterDate(ctx context.Context, date time.Time) ([]UserDoc, error) {
	collection := c.DB.Collection("users")
	filter := bson.M{"last_entered": bson.M{"$gt": date}}
	opts := options.Find().SetSort(bson.M{"last_entered": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users after date: %w", err)
	}
	defer cursor.Close(ctx)

	var users []UserDoc
	if err = cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode active users: %w", err)
	}
	return users, nil
}

func (c *MongoController) ReadSeatLimitsWHITEListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	collectionName := getSeatLimitCollectionName(isMemberSeat, false)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"seat_id": seatId, "user_id": userId}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to read seat limits white list: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []SeatLimitDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode seat limits white list: %w", err)
	}
	return docs, nil
}

func getSeatLimitCollectionName(isMemberSeat, isBlackList bool) string {
	if isMemberSeat {
		if isBlackList {
			return "member_seat_limits_black_list"
		}
		return "member_seat_limits_white_list"
	} else {
		if isBlackList {
			return "general_seat_limits_black_list"
		}
		return "general_seat_limits_white_list"
	}
}

func (c *MongoController) ReadSeatLimitsBLACKListWithSeatIdAndUserId(ctx context.Context, seatId int, userId string, isMemberSeat bool) ([]SeatLimitDoc, error) {
	collectionName := getSeatLimitCollectionName(isMemberSeat, true)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"seat_id": seatId, "user_id": userId}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to read seat limits black list: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []SeatLimitDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode seat limits black list: %w", err)
	}
	return docs, nil
}

func (c *MongoController) CreateSeatLimitInWHITEList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error {
	collectionName := getSeatLimitCollectionName(isMemberSeat, false)
	collection := c.DB.Collection(collectionName)
	seatLimitDoc := SeatLimitDoc{
		SeatID:    seatId,
		UserID:    userId,
		CreatedAt: createdAt,
		Until:     until,
	}
	_, err := collection.InsertOne(ctx, seatLimitDoc)
	if err != nil {
		return fmt.Errorf("failed to create seat limit in white list: %w", err)
	}
	return nil
}

func (c *MongoController) CreateSeatLimitInBLACKList(ctx context.Context, seatId int, userId string, createdAt, until time.Time, isMemberSeat bool) error {
	collectionName := getSeatLimitCollectionName(isMemberSeat, true)
	collection := c.DB.Collection(collectionName)
	seatLimitDoc := SeatLimitDoc{
		SeatID:    seatId,
		UserID:    userId,
		CreatedAt: createdAt,
		Until:     until,
	}
	_, err := collection.InsertOne(ctx, seatLimitDoc)
	if err != nil {
		return fmt.Errorf("failed to create seat limit in black list: %w", err)
	}
	return nil
}

func (c *MongoController) Get500SeatLimitsAfterUntilInWHITEList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatLimitDoc, error) {
	collectionName := getSeatLimitCollectionName(isMemberSeat, false)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"until": bson.M{"$gt": thresholdTime}}
	opts := options.Find().SetSort(bson.M{"until": 1}).SetLimit(500)

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get seat limits after until in white list: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []SeatLimitDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode seat limits after until in white list: %w", err)
	}
	return docs, nil
}

func (c *MongoController) Get500SeatLimitsAfterUntilInBLACKList(ctx context.Context, thresholdTime time.Time, isMemberSeat bool) ([]SeatLimitDoc, error) {
	collectionName := getSeatLimitCollectionName(isMemberSeat, true)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"until": bson.M{"$gt": thresholdTime}}
	opts := options.Find().SetSort(bson.M{"until": 1}).SetLimit(500)

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get seat limits after until in black list: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []SeatLimitDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode seat limits after until in black list: %w", err)
	}
	return docs, nil
}

func (c *MongoController) DeleteSeatLimitInWHITEList(ctx context.Context, docId string, isMemberSeat bool) error {
	collectionName := getSeatLimitCollectionName(isMemberSeat, false)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"_id": docId}

	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete seat limit %s from white list: %w", docId, err)
	}
	return nil
}

func (c *MongoController) DeleteSeatLimitInBLACKList(ctx context.Context, docId string, isMemberSeat bool) error {
	collectionName := getSeatLimitCollectionName(isMemberSeat, true)
	collection := c.DB.Collection(collectionName)
	filter := bson.M{"_id": docId}

	_, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete seat limit %s from black list: %w", docId, err)
	}
	return nil
}

func (c *MongoController) ReadAllMenuDocsOrderByCode(ctx context.Context) ([]MenuDoc, error) {
	collection := c.DB.Collection("menus")
	opts := options.Find().SetSort(bson.M{"code": 1})

	cursor, err := collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to read all menu documents: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []MenuDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("failed to decode menu documents: %w", err)
	}
	return docs, nil
}

func (c *MongoController) CountUserOrdersOfTheDay(ctx context.Context, userId string, date time.Time) (int64, error) {
	collection := c.DB.Collection("order_history")
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour).Add(-time.Nanosecond) // End of day

	filter := bson.M{
		"user_id": userId,
		"created_at": bson.M{
			"$gte": startOfDay,
			"$lte": endOfDay,
		},
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count user orders of the day: %w", err)
	}
	return count, nil
}

func (c *MongoController) CreateOrderHistoryDoc(ctx context.Context, orderHistoryDoc OrderHistoryDoc) error {
	collection := c.DB.Collection("order_history")
	_, err := collection.InsertOne(ctx, orderHistoryDoc)
	if err != nil {
		return fmt.Errorf("failed to create order history document: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateWorkNameTrend(ctx context.Context, workNameTrend WorkNameTrendDoc) error {
	collection := c.DB.Collection("work_name_trends")
	filter := bson.M{"work_name": workNameTrend.WorkName}
	update := bson.M{"$set": workNameTrend}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update work name trend: %w", err)
	}
	return nil
}

func (c *MongoController) GetAllUserDocRefs(ctx context.Context) ([]string, error) {
	collection := c.DB.Collection("users")
	cursor, err := collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, fmt.Errorf("failed to get all user document references: %w", err)
	}
	defer cursor.Close(ctx)

	var userIDs []string
	for cursor.Next(ctx) {
		var result struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode user ID: %w", err)
		}
		userIDs = append(userIDs, result.ID)
	}
	return userIDs, nil
}

func (c *MongoController) GetAllNonDailyZeroUserDocs(ctx context.Context) ([]UserDoc, error) {
	collection := c.DB.Collection("users")
	filter := bson.M{"daily_total_study_sec": bson.M{"$ne": 0}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get all non-daily zero user documents: %w", err)
	}
	defer cursor.Close(ctx)

	var users []UserDoc
	if err = cursor.All(ctx, &users); err != nil {
		return nil, fmt.Errorf("failed to decode non-daily zero user documents: %w", err)
	}
	return users, nil
}

func (c *MongoController) ResetDailyTotalStudyTime(ctx context.Context, userRef *firestore.DocumentRef) error {
	userId := userRef.ID
	collection := c.DB.Collection("users")
	filter := bson.M{"_id": userId}
	update := bson.M{"$set": bson.M{"daily_total_study_sec": 0}}

	_, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to reset daily total study time for user %s: %w", userId, err)
	}
	return nil
}

func (c *MongoController) UpdateLastResetDailyTotalStudyTime(ctx context.Context, timestamp time.Time) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "last_reset_daily_total_study_time"}
	update := bson.M{"$set": bson.M{"value": timestamp}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update last reset daily total study time: %w", err)
	}
	return nil
}


func (c *MongoController) UpdateLastLongTimeSittingChecked(ctx context.Context, timestamp time.Time) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "last_long_time_sitting_checked"}
	update := bson.M{"$set": bson.M{"value": timestamp}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update last long time sitting checked: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateLastTransferCollectionHistoryBigquery(ctx context.Context, timestamp time.Time) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "last_transfer_collection_history_bigquery"}
	update := bson.M{"$set": bson.M{"value": timestamp}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update last transfer collection history bigquery: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateDesiredMaxSeats(ctx context.Context, desiredMaxSeats int) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "desired_max_seats"}
	update := bson.M{"$set": bson.M{"value": desiredMaxSeats}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update desired max seats: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateDesiredMemberMaxSeats(ctx context.Context, desiredMemberMaxSeats int) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "desired_member_max_seats"}
	update := bson.M{"$set": bson.M{"value": desiredMemberMaxSeats}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update desired member max seats: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateMaxSeats(ctx context.Context, maxSeats int) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "max_seats"}
	update := bson.M{"$set": bson.M{"value": maxSeats}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update max seats: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateMemberMaxSeats(ctx context.Context, memberMaxSeats int) error {
	collection := c.DB.Collection("configs")
	filter := bson.M{"_id": "member_max_seats"}
	update := bson.M{"$set": bson.M{"value": memberMaxSeats}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update member max seats: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateAccessTokenOfChannelCredential(ctx context.Context, accessToken string, expireDate time.Time) error {
	collection := c.DB.Collection("credentials")
	filter := bson.M{"_id": "channel_credential"}
	update := bson.M{"$set": bson.M{
		"access_token": accessToken,
		"expire_date":  expireDate,
	}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update access token of channel credential: %w", err)
	}
	return nil
}

func (c *MongoController) UpdateAccessTokenOfBotCredential(ctx context.Context, accessToken string, expireDate time.Time) error {
	collection := c.DB.Collection("credentials")
	filter := bson.M{"_id": "bot_credential"}
	update := bson.M{"$set": bson.M{
		"access_token": accessToken,
		"expire_date":  expireDate,
	}}
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update access token of bot credential: %w", err)
	}
	return nil
}
