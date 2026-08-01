package mypage

import (
	"context"

	"app.modules/core/repository"
)

type Store interface {
	ReadUser(ctx context.Context, youtubeChannelID string) (repository.UserDoc, error)
	ReadSeatByUserID(ctx context.Context, youtubeChannelID string, isMemberSeat bool) (repository.SeatDoc, error)
}

type RepositoryStore struct {
	repo repository.Repository
}

func NewRepositoryStore(repo repository.Repository) *RepositoryStore {
	return &RepositoryStore{
		repo: repo,
	}
}

func (s *RepositoryStore) ReadUser(ctx context.Context, youtubeChannelID string) (repository.UserDoc, error) {
	return s.repo.ReadUser(ctx, nil, youtubeChannelID)
}

func (s *RepositoryStore) ReadSeatByUserID(ctx context.Context, youtubeChannelID string, isMemberSeat bool) (repository.SeatDoc, error) {
	return s.repo.ReadSeatWithUserID(ctx, youtubeChannelID, isMemberSeat)
}
