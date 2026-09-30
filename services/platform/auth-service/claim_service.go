package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/combi/pkg/domain/auth"
)

type claimService struct {
	repo           auth.Repository
	accountService auth.AccountService
}

func NewClaimService(repo auth.Repository, acc auth.AccountService) auth.ClaimService {
	return &claimService{repo: repo, accountService: acc}
}

func (s *claimService) Search(ctx context.Context, identifier string) (bool, *auth.FoundUser, error) {
	acc, err := s.repo.GetAccountByIdentifier(ctx, identifier)
	if err != nil {
		return false, nil, nil // Return false instead of error to prevent enumeration
	}

	return true, &auth.FoundUser{
		ID:          acc.ID,
		FirstName:   "Member", // In a real scenario, fetch this from User Service
		MaskedEmail: maskEmail(acc.Email.String()),
	}, nil
}

func (s *claimService) SendActivationCode(ctx context.Context, accountID uuid.UUID) error {
	acc, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	// Re-use the existing verification logic
	return s.accountService.RequestEmailVerification(ctx, acc.Email)
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}
	return parts[0][:2] + "****@" + parts[1]
}
