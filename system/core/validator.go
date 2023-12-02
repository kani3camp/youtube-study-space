package core

import (
	"app.modules/core/i18n"
	"app.modules/core/myfirestore"
	"app.modules/core/studyspaceerror"
	"app.modules/core/utils"
	"github.com/pkg/errors"
)

func (s *System) ValidateCommand(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	switch command.CommandType {
	case utils.In:
		return s.ValidateIn(command)
	case utils.Out:
		return studyspaceerror.NewNil()
	case utils.Info:
		return s.ValidateInfo(command)
	case utils.My:
		return s.ValidateMy(command)
	case utils.Change:
		// seatStateに依存するためChange()の中で行う。
		return studyspaceerror.NewNil()
	case utils.Seat:
		return s.ValidateSeat(command)
	case utils.Report:
		return s.ValidateReport(command)
	case utils.Kick:
		return s.ValidateKick(command)
	case utils.Check:
		return s.ValidateCheck(command)
	case utils.Block:
		return s.ValidateBlock(command)
	case utils.More:
		return s.ValidateMore(command)
	case utils.Break:
		return s.ValidateBreak(command)
	case utils.Resume:
		return s.ValidateResume(command)
	case utils.Rank:
		return studyspaceerror.NewNil()
	}
	return studyspaceerror.NewNil()
}

func (s *System) ValidateIn(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.In {
		return studyspaceerror.InvalidParsedCommand.New("this is not a In command.")
	}

	// 作業時間の値
	inputWorkMin := command.InOption.MinutesAndWorkName.DurationMin
	if inputWorkMin != 0 {
		expect := s.Configs.Constants.MinWorkTimeMin <= inputWorkMin && inputWorkMin <= s.Configs.Constants.MaxWorkTimeMin
		if !expect {
			return studyspaceerror.InvalidCommand.New(i18n.T("validate:invalid-work-time-range", s.Configs.Constants.MinWorkTimeMin, s.Configs.Constants.MaxWorkTimeMin))
		}
	}
	// 席番号
	if command.InOption.IsSeatIdSet {
		if command.InOption.SeatId < 0 {
			return studyspaceerror.InvalidCommand.New(i18n.T("validate:negative-seat-id"))
		}
	}

	// 作業名は特に制限はない
	// pass

	return studyspaceerror.NewNil()
}

func (s *System) ValidateInfo(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Info {
		return studyspaceerror.InvalidParsedCommand.New("this is not a Info command.")
	}

	// pass

	return studyspaceerror.NewNil()
}

func (s *System) ValidateMy(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.My {
		return studyspaceerror.InvalidParsedCommand.New("this is not a My command.")
	}

	var isRankVisibleSet, isDefaultStudyMinSet, isFavoriteColorSet bool

	for _, option := range command.MyOptions {
		switch option.Type {
		case utils.RankVisible:
			if isRankVisibleSet {
				return studyspaceerror.InvalidParsedCommand.New("more than 2 RankVisible options.")
			}
			isRankVisibleSet = true
		case utils.DefaultStudyMin:
			if isDefaultStudyMinSet {
				return studyspaceerror.InvalidParsedCommand.New("more than 2 DefaultStudyMin options.")
			}
			inputDefaultStudyMin := option.IntValue
			if inputDefaultStudyMin != 0 {
				expect := s.Configs.Constants.MinWorkTimeMin <= inputDefaultStudyMin && inputDefaultStudyMin <= s.Configs.Constants.MaxWorkTimeMin
				if !expect {
					return studyspaceerror.InvalidCommand.New(i18n.T("validate:invalid-work-time-range", s.Configs.Constants.MinWorkTimeMin, s.Configs.Constants.MaxWorkTimeMin))
				}
			}
			isDefaultStudyMinSet = true
		case utils.FavoriteColor:
			if isFavoriteColorSet {
				return studyspaceerror.InvalidParsedCommand.New("more than 2 FavoriteColor options.")
			}
			expect := utils.IsIncludedInColorNames(option.StringValue) || option.StringValue == ""
			if !expect {
				return studyspaceerror.InvalidParsedCommand.New(i18n.T("validate:invalid-favorite-color-option", utils.FavoriteColorMyOptionPrefix))
			}
			isFavoriteColorSet = true
		default:
			return studyspaceerror.InvalidParsedCommand.New("there is an unknown option in command.MyOptions")
		}
	}
	return studyspaceerror.NewNil()
}

func (s *System) ValidateSeat(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Seat {
		return studyspaceerror.InvalidParsedCommand.New("this is not a Seat command.")
	}

	// pass

	return studyspaceerror.NewNil()
}

func (s *System) ValidateKick(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Kick {
		return studyspaceerror.InvalidParsedCommand.New("this is not a Kick command.")
	}

	// 指定座席番号
	if command.KickOption.SeatId <= 0 {
		return studyspaceerror.InvalidCommand.New(i18n.T("validate:non-one-or-more-seat-id"))
	}

	return studyspaceerror.NewNil()
}

func (s *System) ValidateCheck(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Check {
		return studyspaceerror.InvalidParsedCommand.New("this is not a Check command.")
	}

	// 指定座席番号
	if command.CheckOption.SeatId <= 0 {
		return studyspaceerror.InvalidCommand.New(i18n.T("validate:non-one-or-more-seat-id"))
	}

	return studyspaceerror.NewNil()
}

func (s *System) ValidateBlock(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Block {
		return studyspaceerror.InvalidParsedCommand.New("this is not a Block command.")
	}

	// 指定座席番号
	if command.BlockOption.SeatId <= 0 {
		return studyspaceerror.InvalidCommand.New("席番号は1以上にしてください。")
	}

	return studyspaceerror.NewNil()
}

func (s *System) ValidateReport(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Report {
		return studyspaceerror.InvalidParsedCommand.New("this is not a Report command.")
	}

	// 空欄でないか
	if command.ReportOption.Message == "" {
		return studyspaceerror.InvalidCommand.New(i18n.T("validate:parse:missing-message", utils.ReportCommand))
	}

	return studyspaceerror.NewNil()
}

func (s *System) ValidateChange(command utils.CommandDetails, seatState myfirestore.SeatState) error {
	if command.CommandType != utils.Change {
		return errors.New("this is not a Change command.")
	}

	// オプションが1つ以上指定されているか
	if command.ChangeOption.NumOptionsSet() == 0 {
		return errors.New(i18n.T("validate:missing-option"))
	}

	switch seatState {
	case myfirestore.WorkState:
		// 作業内容
		// pass

		// 入室時間
		if command.ChangeOption.IsDurationMinSet {
			inputDurationMin := command.ChangeOption.DurationMin
			expect := s.Configs.Constants.MinWorkTimeMin <= inputDurationMin && inputDurationMin <= s.Configs.Constants.MaxWorkTimeMin
			if !expect {
				return errors.New(i18n.T("validate:invalid-work-time-range", s.Configs.Constants.MinWorkTimeMin, s.Configs.Constants.MaxWorkTimeMin))
			}
		}
	case myfirestore.BreakState:
		// 休憩内容
		// pass

		// 休憩時間
		if command.ChangeOption.IsDurationMinSet {
			inputDurationMin := command.ChangeOption.DurationMin
			expect := s.Configs.Constants.MinBreakDurationMin <= inputDurationMin && inputDurationMin <= s.Configs.Constants.MaxBreakDurationMin
			if !expect {
				return errors.New(i18n.T("validate:invalid-break-time-range", s.Configs.Constants.MinBreakDurationMin, s.Configs.Constants.MaxBreakDurationMin))
			}
		}
	}

	return nil
}

func (s *System) ValidateMore(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.More {
		return studyspaceerror.InvalidParsedCommand.New("this is not a More command.")
	}

	// 時間オプション
	if command.MoreOption.DurationMin <= 0 {
		return studyspaceerror.InvalidCommand.New(i18n.T("validate:non-one-or-more-extended-time"))
	}

	return studyspaceerror.NewNil()
}

func (s *System) ValidateBreak(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Break {
		return studyspaceerror.InvalidParsedCommand.New("this is not a More command.")
	}

	// 休憩内容
	// pass

	// 休憩時間
	if command.BreakOption.IsDurationMinSet {
		inputDurationMin := command.BreakOption.DurationMin
		expect := s.Configs.Constants.MinBreakDurationMin <= inputDurationMin && inputDurationMin <= s.Configs.Constants.MaxBreakDurationMin
		if !expect {
			return studyspaceerror.InvalidCommand.New(i18n.T("validate:invalid-break-time-range", s.Configs.Constants.MinBreakDurationMin, s.Configs.Constants.MaxBreakDurationMin))
		}
	}

	return studyspaceerror.NewNil()
}

func (s *System) ValidateResume(command utils.CommandDetails) studyspaceerror.StudySpaceError {
	if command.CommandType != utils.Resume {
		return studyspaceerror.InvalidParsedCommand.New("this is not a Resume command.")
	}

	// 作業名
	// pass

	return studyspaceerror.NewNil()
}
