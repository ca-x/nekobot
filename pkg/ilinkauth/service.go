package ilinkauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	wxtypes "nekobot/pkg/wechat/types"
)

type loginClient interface {
	FetchQRCode(ctx context.Context) (*wxtypes.QRCodeResponse, error)
	CheckQRStatus(ctx context.Context, qrcode string) (*wxtypes.QRStatusResponse, error)
}

// Service drives iLink QR binding on top of Store.
type Service struct {
	store *Store
	login loginClient
}

// NewService creates a shared iLink auth service.
func NewService(store *Store, login loginClient) *Service {
	return &Service{
		store: store,
		login: login,
	}
}

// StartBinding creates or replaces the current user's bind session.
func (s *Service) StartBinding(ctx context.Context, userID string) (*BindSession, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if s.login == nil {
		return nil, fmt.Errorf("login client is nil")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}

	qrResp, err := s.login.FetchQRCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch QR code: %w", err)
	}

	session := BindSession{
		UserID:        userID,
		QRCode:        qrResp.QRCode,
		QRCodeContent: qrResp.QRCodeImgContent,
		Status:        BindStatusPending,
	}
	if err := s.store.SaveBindSession(session); err != nil {
		return nil, err
	}
	return s.store.LoadBindSession(userID)
}

// PollBinding refreshes the current user's bind session from iLink status.
func (s *Service) PollBinding(ctx context.Context, userID string) (*BindSession, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if s.login == nil {
		return nil, fmt.Errorf("login client is nil")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}

	session, err := s.store.LoadBindSession(userID)
	if err != nil {
		return nil, err
	}
	if session == nil || strings.TrimSpace(session.QRCode) == "" {
		return nil, fmt.Errorf("no active ilink binding")
	}

	statusResp, err := s.login.CheckQRStatus(ctx, session.QRCode)
	if err != nil {
		session.Status = BindStatusFailed
		session.Error = err.Error()
		if saveErr := s.store.SaveBindSession(*session); saveErr != nil {
			return nil, saveErr
		}
		return nil, fmt.Errorf("check QR status: %w", err)
	}

	session.Error = ""
	switch statusResp.Status {
	case wxtypes.QRStatusConfirmed:
		session.Status = BindStatusConfirmed
		session.BotID = statusResp.ILinkBotID
		session.ILinkUserID = statusResp.ILinkUserID
		binding := &Binding{
			UserID: userID,
			Credentials: wxtypes.Credentials{
				BotToken:    statusResp.BotToken,
				ILinkBotID:  statusResp.ILinkBotID,
				BaseURL:     statusResp.BaseURL,
				ILinkUserID: statusResp.ILinkUserID,
			},
		}
		if err := s.store.SaveBinding(binding); err != nil {
			return nil, err
		}
	case wxtypes.QRStatusScanned:
		session.Status = BindStatusScanned
	case wxtypes.QRStatusExpired:
		session.Status = BindStatusExpired
	default:
		session.Status = BindStatusPending
	}

	if err := s.store.SaveBindSession(*session); err != nil {
		return nil, err
	}
	return s.store.LoadBindSession(userID)
}

// GetBinding returns the current user's saved binding.
func (s *Service) GetBinding(userID string) (*Binding, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	return s.store.LoadBinding(userID)
}

// ListBindings returns all persisted bindings.
func (s *Service) ListBindings() ([]*Binding, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	return s.store.ListBindings()
}

// DeleteBinding removes both saved binding and current bind session.
func (s *Service) DeleteBinding(userID string) error {
	if s.store == nil {
		return fmt.Errorf("store is nil")
	}
	if err := s.store.ClearBindSession(userID); err != nil {
		return err
	}
	if err := s.store.DeleteBinding(userID); err != nil && !errors.Is(err, ErrBindingNotFound) {
		return err
	}
	return nil
}

// SaveBinding persists one binding directly.
func (s *Service) SaveBinding(binding *Binding) error {
	if s.store == nil {
		return fmt.Errorf("store is nil")
	}
	return s.store.SaveBinding(binding)
}

// LoadBindSession returns the user's current bind session, if any.
func (s *Service) LoadBindSession(userID string) (*BindSession, error) {
	if s.store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	return s.store.LoadBindSession(userID)
}

// SyncStatePath returns the channel sync-state path for the user's bound bot.
func (s *Service) SyncStatePath(userID, botID string) string {
	if s.store == nil {
		return ""
	}
	return s.store.SyncStatePath(userID, botID)
}
