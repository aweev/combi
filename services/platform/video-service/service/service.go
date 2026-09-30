package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/ion-sfu/pkg/sfu"
	"github.com/pion/webrtc/v4"
	"github.com/combi/pkg/log"
	"github.com/combi/video/domain"
)

type service struct {
	repo   domain.Repository
	logger log.Logger

	// SFU Engine
	sfu *sfu.SFU

	// Config
	iceServers []domain.IceServer

	mu    sync.RWMutex
	peers map[string]*webrtc.PeerConnection
}

func New(repo domain.Repository, logger log.Logger, iceServers []domain.IceServer) domain.Service {
	// Initialize SFU
	s := sfu.NewSFU(sfu.Config{
		WebRTC: sfu.WebRTCConfig{
			ICEPortRange: []uint16{5000, 5100},
		},
	})

	return &service{
		repo:       repo,
		logger:     logger,
		sfu:        s,
		iceServers: iceServers,
		peers:      make(map[string]*webrtc.PeerConnection),
	}
}

func (s *service) CreateRoom(ctx context.Context, params domain.CreateRoomParams) (*domain.Room, error) {
	room := &domain.Room{
		ID:          uuid.New(),
		Title:       params.Title,
		Description: params.Description,
		Type:        params.Type,
		Status:      domain.RoomStatusActive,
		RefType:     params.RefType,
		RefID:       params.RefID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.StoreRoom(ctx, room); err != nil {
		return nil, fmt.Errorf("failed to store room: %w", err)
	}

	return room, nil
}

func (s *service) GetRoom(ctx context.Context, id uuid.UUID) (*domain.Room, error) {
	return s.repo.FindRoomByID(ctx, id)
}

func (s *service) JoinRoom(ctx context.Context, roomID uuid.UUID, userID uuid.UUID) (*domain.JoinResponse, error) {
	room, err := s.repo.FindRoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}

	if room.Status != domain.RoomStatusActive {
		return nil, fmt.Errorf("room is not active")
	}

	participant, err := s.repo.FindParticipant(ctx, roomID, userID)
	if err != nil {
		// Create new participant
		participant = &domain.Participant{
			ID:       uuid.New(),
			RoomID:   roomID,
			UserID:   userID,
			JoinedAt: time.Now(),
		}
		if err := s.repo.StoreParticipant(ctx, participant); err != nil {
			return nil, err
		}
	}

	return &domain.JoinResponse{
		Room:        room,
		Participant: participant,
		IceServers:  s.iceServers,
	}, nil
}

func (s *service) EndRoom(ctx context.Context, roomID uuid.UUID) error {
	room, err := s.repo.FindRoomByID(ctx, roomID)
	if err != nil {
		return err
	}

	room.Status = domain.RoomStatusEnded
	room.EndTime = new(time.Time)
	*room.EndTime = time.Now()
	room.UpdatedAt = time.Now()

	return s.repo.UpdateRoom(ctx, room)
}

func (s *service) HandleOffer(ctx context.Context, roomID uuid.UUID, userID uuid.UUID, offer domain.SessionDescription) (*domain.SessionDescription, error) {
	s.logger.Info("Handling WebRTC offer",
		log.String("room_id", roomID.String()),
		log.String("user_id", userID.String()),
	)

	// 1. Setup PeerConnection
	config := webrtc.Configuration{}
	for _, ice := range s.iceServers {
		config.ICEServers = append(config.ICEServers, webrtc.ICEServer{
			URLs:       ice.URLs,
			Username:   ice.Username,
			Credential: ice.Credential,
		})
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// 2. Store Peer
	peerKey := fmt.Sprintf("%s-%s", roomID, userID)
	s.mu.Lock()
	s.peers[peerKey] = pc
	s.mu.Unlock()

	// 3. Set Remote Description
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offer.SDP,
	}); err != nil {
		return nil, err
	}

	// 4. Create Answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	return &domain.SessionDescription{
		Type: "answer",
		SDP:  answer.SDP,
	}, nil
}

func (s *service) HandleAnswer(ctx context.Context, roomID uuid.UUID, userID uuid.UUID, answer domain.SessionDescription) error {
	peerKey := fmt.Sprintf("%s-%s", roomID, userID)
	s.mu.RLock()
	pc, ok := s.peers[peerKey]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("peer not found")
	}

	return pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer.SDP,
	})
}

func (s *service) HandleIceCandidate(ctx context.Context, roomID uuid.UUID, userID uuid.UUID, candidate domain.IceCandidate) error {
	peerKey := fmt.Sprintf("%s-%s", roomID, userID)
	s.mu.RLock()
	pc, ok := s.peers[peerKey]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("peer not found")
	}

	return pc.AddICECandidate(webrtc.ICECandidateInit{
		Candidate:     candidate.Candidate,
		SDPMid:        candidate.SDPMid,
		SDPMLineIndex: candidate.SDPMLineIndex,
	})
}
