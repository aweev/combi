package domain

import (
	"context"

	"github.com/google/uuid"
)

type Service interface {
	CreateRoom(ctx context.Context, params CreateRoomParams) (*Room, error)
	GetRoom(ctx context.Context, id uuid.UUID) (*Room, error)
	JoinRoom(ctx context.Context, roomID uuid.UUID, userID uuid.UUID) (*JoinResponse, error)
	EndRoom(ctx context.Context, roomID uuid.UUID) error

	// Signaling
	HandleOffer(ctx context.Context, roomID uuid.UUID, userID uuid.UUID, offer SessionDescription) (*SessionDescription, error)
	HandleAnswer(ctx context.Context, roomID uuid.UUID, userID uuid.UUID, answer SessionDescription) error
	HandleIceCandidate(ctx context.Context, roomID uuid.UUID, userID uuid.UUID, candidate IceCandidate) error
}

type Repository interface {
	StoreRoom(ctx context.Context, room *Room) error
	FindRoomByID(ctx context.Context, id uuid.UUID) (*Room, error)
	FindActiveRoomByRef(ctx context.Context, refType string, refID uuid.UUID) (*Room, error)
	UpdateRoom(ctx context.Context, room *Room) error

	StoreParticipant(ctx context.Context, p *Participant) error
	FindParticipant(ctx context.Context, roomID uuid.UUID, userID uuid.UUID) (*Participant, error)
	UpdateParticipant(ctx context.Context, p *Participant) error
	ListParticipants(ctx context.Context, roomID uuid.UUID) ([]Participant, error)
}

type CreateRoomParams struct {
	Title       string
	Description string
	Type        RoomType
	RefType     string
	RefID       uuid.UUID
	CreatorID   uuid.UUID
}

type JoinResponse struct {
	Room        *Room
	Participant *Participant
	IceServers  []IceServer
}

type IceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}
