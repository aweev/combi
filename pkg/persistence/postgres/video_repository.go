package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/combi/pkg/domain/video"
	"github.com/combi/pkg/log"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Ensure implementation satisfies interface
var _ video.Repository = (*VideoRepository)(nil)

type VideoRepository struct {
	db     *sqlx.DB
	logger log.Logger
	sq     squirrel.StatementBuilderType
}

func NewVideoRepository(db *sqlx.DB, logger log.Logger) *VideoRepository {
	return &VideoRepository{
		db:     db,
		logger: logger.With(log.String("component", "video_repository")),
		sq:     squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *VideoRepository) StoreRoom(ctx context.Context, room *video.Room) error {
	query, args, err := r.sq.Insert("video_rooms").
		Columns("id", "title", "description", "type", "status", "ref_type", "ref_id", "max_participants", "is_recorded", "start_time", "end_time", "created_at", "updated_at").
		Values(room.ID, room.Title, room.Description, room.Type, room.Status, room.RefType, room.RefID, room.MaxParticipants, room.IsRecorded, room.StartTime, room.EndTime, room.CreatedAt, room.UpdatedAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *VideoRepository) FindRoomByID(ctx context.Context, id uuid.UUID) (*video.Room, error) {
	query, args, _ := r.sq.Select("*").From("video_rooms").Where(squirrel.Eq{"id": id}).ToSql()
	var room video.Room
	if err := sqlx.GetContext(ctx, r.db, &room, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, err
	}
	return &room, nil
}

func (r *VideoRepository) FindActiveRoomByRef(ctx context.Context, refType string, refID uuid.UUID) (*video.Room, error) {
	query, args, _ := r.sq.Select("*").From("video_rooms").
		Where(squirrel.Eq{"ref_type": refType, "ref_id": refID, "status": video.RoomStatusActive}).
		ToSql()
	var room video.Room
	if err := sqlx.GetContext(ctx, r.db, &room, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &room, nil
}

func (r *VideoRepository) UpdateRoom(ctx context.Context, room *video.Room) error {
	query, args, err := r.sq.Update("video_rooms").
		Set("title", room.Title).
		Set("description", room.Description).
		Set("status", room.Status).
		Set("end_time", room.EndTime).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": room.ID}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *VideoRepository) StoreParticipant(ctx context.Context, p *video.Participant) error {
	query, args, err := r.sq.Insert("video_participants").
		Columns("id", "room_id", "user_id", "is_host", "is_muted", "video_off", "joined_at", "left_at").
		Values(p.ID, p.RoomID, p.UserID, p.IsHost, p.IsMuted, p.VideoOff, p.JoinedAt, p.LeftAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *VideoRepository) FindParticipant(ctx context.Context, roomID uuid.UUID, userID uuid.UUID) (*video.Participant, error) {
	query, args, _ := r.sq.Select("*").From("video_participants").
		Where(squirrel.Eq{"room_id": roomID, "user_id": userID}).
		ToSql()
	var p video.Participant
	if err := sqlx.GetContext(ctx, r.db, &p, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("participant not found")
		}
		return nil, err
	}
	return &p, nil
}

func (r *VideoRepository) UpdateParticipant(ctx context.Context, p *video.Participant) error {
	query, args, err := r.sq.Update("video_participants").
		Set("is_muted", p.IsMuted).
		Set("video_off", p.VideoOff).
		Set("left_at", p.LeftAt).
		Where(squirrel.Eq{"id": p.ID}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *VideoRepository) ListParticipants(ctx context.Context, roomID uuid.UUID) ([]video.Participant, error) {
	query, args, _ := r.sq.Select("*").From("video_participants").Where(squirrel.Eq{"room_id": roomID}).ToSql()
	var participants []video.Participant
	if err := sqlx.SelectContext(ctx, r.db, &participants, query, args...); err != nil {
		return nil, err
	}
	return participants, nil
}
