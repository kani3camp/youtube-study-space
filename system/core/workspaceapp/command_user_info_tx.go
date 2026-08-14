package workspaceapp

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/repository"
	"app.modules/core/timeutil"
	"app.modules/core/utils"
)

// buildUserInfoReplyTx performs the !info reads inside a caller-owned
// Firestore transaction and returns the reply instead of posting it. Legacy
// ShowUserInfo wraps this helper in its own transaction; the durable Inbox
// worker can compose it between Begin/Finalize in the same message transaction.
func (app *WorkspaceApp) buildUserInfoReplyTx(
	ctx context.Context,
	tx *firestore.Transaction,
	infoOption *utils.InfoOption,
) (string, error) {
	totalStudyDuration, dailyTotalStudyDuration, err := app.GetUserRealtimeTotalStudyDurations(ctx, tx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("in app.GetUserRealtimeTotalStudyDurations(): %w", err)
	}

	userDoc, err := app.Repository.ReadUser(ctx, tx, app.ProcessedUserID)
	if err != nil {
		return "", fmt.Errorf("in app.Repository.ReadUser: %w", err)
	}

	return app.buildUserInfoReply(userDoc, totalStudyDuration, dailyTotalStudyDuration, infoOption), nil
}

// buildUserInfoReply renders an !info response from already-read domain data.
// It performs no repository access and no external delivery. The durable Inbox
// path can therefore render the first-use response before staging User creation,
// avoiding Firestore's read-after-write transaction restriction.
func (app *WorkspaceApp) buildUserInfoReply(
	userDoc repository.UserDoc,
	totalStudyDuration time.Duration,
	dailyTotalStudyDuration time.Duration,
	infoOption *utils.InfoOption,
) string {
	dailyTotalTimeStr := timeutil.DurationToString(dailyTotalStudyDuration)
	totalTimeStr := timeutil.DurationToString(totalStudyDuration)
	replyMessage := i18nmsg.CommandUserInfoBase(app.ProcessedUserDisplayName, dailyTotalTimeStr, totalTimeStr)

	if userDoc.RankVisible {
		replyMessage += i18nmsg.CommandUserInfoRank(userDoc.RankPoint)
	}

	if infoOption.ShowDetails {
		switch userDoc.RankVisible {
		case true:
			replyMessage += i18nmsg.CommandUserInfoRankOn()
		case false:
			replyMessage += i18nmsg.CommandUserInfoRankOff()
		}

		if userDoc.IsContinuousActive {
			continuousActiveDays := int(app.currentTime().Sub(userDoc.CurrentActivityStateStarted).Hours() / 24)
			replyMessage += i18nmsg.CommandUserInfoRankOnContinuous(continuousActiveDays+1, continuousActiveDays)
		}

		if userDoc.DefaultStudyMin == 0 {
			replyMessage += i18nmsg.CommandUserInfoDefaultWorkOff()
		} else {
			replyMessage += i18nmsg.CommandUserInfoDefaultWork(userDoc.DefaultStudyMin)
		}

		if userDoc.FavoriteColor == "" {
			replyMessage += i18nmsg.CommandUserInfoFavoriteColorOff()
		} else {
			replyMessage += i18nmsg.CommandUserInfoFavoriteColor(utils.ColorCodeToColorName(userDoc.FavoriteColor))
		}

		replyMessage += i18nmsg.CommandUserInfoRegisterDate(userDoc.RegistrationDate.In(timeutil.JapanLocation()).Format("2006年01月02日"))
	}
	return replyMessage
}
