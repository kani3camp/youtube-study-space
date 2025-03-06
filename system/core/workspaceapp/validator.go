package workspaceapp

import (
	"app.modules/core/i18n"
	"app.modules/core/repository"
	"app.modules/core/utils"
	"github.com/pkg/errors"
)

func (app *WorkspaceApp) ValidateCommand(command utils.CommandDetails) string {
	switch command.CommandType {
	case utils.In:
		return app.ValidateIn(command)
	case utils.Out:
		return ""
	case utils.Info:
		return app.ValidateInfo(command)
	case utils.My:
		return app.ValidateMy(command)
	case utils.Change:
		// seatStateに依存するためChange()の中で行う。
		return ""
	case utils.Seat:
		return app.ValidateSeat(command)
	case utils.Report:
		return app.ValidateReport(command)
	case utils.Kick:
		return app.ValidateKick(command)
	case utils.Check:
		return app.ValidateCheck(command)
	case utils.Block:
		return app.ValidateBlock(command)
	case utils.More:
		return app.ValidateMore(command)
	case utils.Break:
		return app.ValidateBreak(command)
	case utils.Resume:
		return app.ValidateResume(command)
	case utils.Rank:
		return ""
	case utils.Order:
		return app.ValidateOrder(command)
	case utils.Clear:
		return ""
	default:
		return ""
	}
}

func (app *WorkspaceApp) ValidateIn(command utils.CommandDetails) string {

	// 作業時間の値
	inputWorkMin := command.InOption.MinutesAndWorkName.DurationMin
	if inputWorkMin != 0 {
		expect := app.Configs.Constants.MinWorkTimeMin <= inputWorkMin && inputWorkMin <= app.Configs.Constants.MaxWorkTimeMin
		if !expect {
			return i18n.T("validate:invalid-work-time-range", app.Configs.Constants.MinWorkTimeMin, app.Configs.Constants.MaxWorkTimeMin)
		}
	}
	// 席番号
	if command.InOption.IsSeatIdSet {
		if command.InOption.SeatId < 0 {
			return i18n.T("validate:negative-seat-id")
		}
	}

	// 作業名は特に制限はない
	// pass

	return ""
}

func (app *WorkspaceApp) ValidateInfo(_ utils.CommandDetails) string {
	// pass

	return ""
}

func (app *WorkspaceApp) ValidateMy(command utils.CommandDetails) string {
	var isRankVisibleSet, isDefaultStudyMinSet, isFavoriteColorSet bool

	for _, option := range command.MyOptions {
		switch option.Type {
		case utils.RankVisible:
			if isRankVisibleSet {
				return "more than 2 RankVisible options."
			}
			isRankVisibleSet = true
		case utils.DefaultStudyMin:
			if isDefaultStudyMinSet {
				return "more than 2 DefaultStudyMin options."
			}
			inputDefaultStudyMin := option.IntValue
			if inputDefaultStudyMin != 0 {
				expect := app.Configs.Constants.MinWorkTimeMin <= inputDefaultStudyMin && inputDefaultStudyMin <= app.Configs.Constants.MaxWorkTimeMin
				if !expect {
					return i18n.T("validate:invalid-work-time-range", app.Configs.Constants.MinWorkTimeMin, app.Configs.Constants.MaxWorkTimeMin)
				}
			}
			isDefaultStudyMinSet = true
		case utils.FavoriteColor:
			if isFavoriteColorSet {
				return "more than 2 FavoriteColor options."
			}
			expect := utils.IsIncludedInColorNames(option.StringValue) || option.StringValue == ""
			if !expect {
				return i18n.T("validate:invalid-favorite-color-option", utils.FavoriteColorMyOptionPrefix)
			}
			isFavoriteColorSet = true
		default:
			return "there is an unknown option in command.MyOptions"
		}
	}
	return ""
}

func (app *WorkspaceApp) ValidateSeat(_ utils.CommandDetails) string {
	// pass

	return ""
}

func (app *WorkspaceApp) ValidateKick(command utils.CommandDetails) string {
	// 指定座席番号
	if command.KickOption.SeatId <= 0 {
		return i18n.T("validate:non-one-or-more-seat-id")
	}

	return ""
}

func (app *WorkspaceApp) ValidateCheck(command utils.CommandDetails) string {
	// 指定座席番号
	if command.CheckOption.SeatId <= 0 {
		return i18n.T("validate:non-one-or-more-seat-id")
	}

	return ""
}

func (app *WorkspaceApp) ValidateBlock(command utils.CommandDetails) string {
	// 指定座席番号
	if command.BlockOption.SeatId <= 0 {
		return "席番号は1以上にしてください。"
	}

	return ""
}

func (app *WorkspaceApp) ValidateReport(command utils.CommandDetails) string {
	// 空欄でないか
	if command.ReportOption.Message == "" {
		return i18n.T("validate:parse:missing-message", utils.ReportCommand)
	}

	return ""
}

func (app *WorkspaceApp) ValidateChange(changeOption utils.MinWorkOrderOption, seatState repository.SeatState) error {
	// オプションが1つ以上指定されているか
	if changeOption.NumOptionsSet() == 0 {
		return errors.New(i18n.T("validate:missing-option"))
	}

	switch seatState {
	case repository.WorkState:
		// 作業内容
		// pass

		// 入室時間
		if changeOption.IsDurationMinSet {
			inputDurationMin := changeOption.DurationMin
			expect := app.Configs.Constants.MinWorkTimeMin <= inputDurationMin && inputDurationMin <= app.Configs.Constants.MaxWorkTimeMin
			if !expect {
				return errors.New(i18n.T("validate:invalid-work-time-range", app.Configs.Constants.MinWorkTimeMin, app.Configs.Constants.MaxWorkTimeMin))
			}
		}
	case repository.BreakState:
		// 休憩内容
		// pass

		// 休憩時間
		if changeOption.IsDurationMinSet {
			inputDurationMin := changeOption.DurationMin
			expect := app.Configs.Constants.MinBreakDurationMin <= inputDurationMin && inputDurationMin <= app.Configs.Constants.MaxBreakDurationMin
			if !expect {
				return errors.New(i18n.T("validate:invalid-break-time-range", app.Configs.Constants.MinBreakDurationMin, app.Configs.Constants.MaxBreakDurationMin))
			}
		}
	}

	return nil
}

func (app *WorkspaceApp) ValidateMore(command utils.CommandDetails) string {
	// 時間オプション
	if command.MoreOption.IsDurationMinSet && command.MoreOption.DurationMin <= 0 {
		return i18n.T("validate:non-one-or-more-extended-time")
	}

	return ""
}

func (app *WorkspaceApp) ValidateBreak(command utils.CommandDetails) string {
	// 休憩内容
	// pass

	// 休憩時間
	if command.BreakOption.IsDurationMinSet {
		inputDurationMin := command.BreakOption.DurationMin
		expect := app.Configs.Constants.MinBreakDurationMin <= inputDurationMin && inputDurationMin <= app.Configs.Constants.MaxBreakDurationMin
		if !expect {
			return i18n.T("validate:invalid-break-time-range", app.Configs.Constants.MinBreakDurationMin, app.Configs.Constants.MaxBreakDurationMin)
		}
	}

	return ""
}

func (app *WorkspaceApp) ValidateResume(_ utils.CommandDetails) string {
	// 作業名
	// pass

	return ""
}

func (app *WorkspaceApp) ValidateOrder(command utils.CommandDetails) string {
	if !command.OrderOption.ClearFlag {
		num := command.OrderOption.IntValue
		expect := 0 < num && num <= len(app.SortedMenuItems)
		if !expect {
			return i18n.T("validate:invalid-menu-number-range", len(app.SortedMenuItems))
		}
	}

	return ""
}
