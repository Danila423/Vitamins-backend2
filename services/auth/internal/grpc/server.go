package grpc

import (
	"context"
	"errors"

	authv1 "vitamins-backend_2/gen/go/auth/v1"
	"vitamins-backend_2/services/auth/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	authv1.UnimplementedAuthServiceServer
	svc service.ServiceAPI
}

func NewServer(svc service.ServiceAPI) *Server {
	return &Server{svc: svc}
}

func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.TokenPairResponse, error) {
	tp, err := s.svc.Register(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, mapError(err)
	}
	return tokenPairResponse(tp), nil
}

func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.TokenPairResponse, error) {
	tp, err := s.svc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, mapError(err)
	}
	return tokenPairResponse(tp), nil
}

func (s *Server) Refresh(ctx context.Context, req *authv1.RefreshRequest) (*authv1.TokenPairResponse, error) {
	tp, err := s.svc.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, mapError(err)
	}
	return tokenPairResponse(tp), nil
}

func (s *Server) GetProfile(ctx context.Context, req *authv1.GetProfileRequest) (*authv1.UserProfileResponse, error) {
	p, err := s.svc.GetProfile(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return profileResponse(p), nil
}

func (s *Server) UpdateProfile(ctx context.Context, req *authv1.UpdateProfileRequest) (*authv1.UserProfileResponse, error) {
	upd := service.ProfileUpdate{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
	}
	p, err := s.svc.UpdateProfile(ctx, req.GetUserId(), upd)
	if err != nil {
		return nil, mapError(err)
	}
	return profileResponse(p), nil
}

func (s *Server) DeleteAccount(ctx context.Context, req *authv1.DeleteAccountRequest) (*authv1.Empty, error) {
	if err := s.svc.DeleteAccount(ctx, req.GetUserId()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.Empty{}, nil
}

func (s *Server) RequestPasswordReset(ctx context.Context, req *authv1.RequestPasswordResetRequest) (*authv1.Empty, error) {
	if err := s.svc.RequestPasswordReset(ctx, req.GetEmail()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.Empty{}, nil
}

func (s *Server) VerifyPasswordResetCode(ctx context.Context, req *authv1.VerifyPasswordResetCodeRequest) (*authv1.VerifyCodeResponse, error) {
	token, err := s.svc.VerifyPasswordResetCode(ctx, req.GetEmail(), req.GetCode())
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.VerifyCodeResponse{Token: token}, nil
}

func (s *Server) ConfirmPasswordReset(ctx context.Context, req *authv1.ConfirmPasswordResetRequest) (*authv1.Empty, error) {
	if err := s.svc.ConfirmPasswordReset(ctx, req.GetResetToken(), req.GetPassword(), req.GetPasswordConfirm()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.Empty{}, nil
}

func (s *Server) RequestPasswordChange(ctx context.Context, req *authv1.RequestPasswordChangeRequest) (*authv1.Empty, error) {
	if err := s.svc.RequestPasswordChange(ctx, req.GetUserId()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.Empty{}, nil
}

func (s *Server) VerifyPasswordChangeCode(ctx context.Context, req *authv1.VerifyPasswordChangeCodeRequest) (*authv1.VerifyCodeResponse, error) {
	token, err := s.svc.VerifyPasswordChangeCode(ctx, req.GetUserId(), req.GetCode())
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.VerifyCodeResponse{Token: token}, nil
}

func (s *Server) ConfirmPasswordChange(ctx context.Context, req *authv1.ConfirmPasswordChangeRequest) (*authv1.Empty, error) {
	if err := s.svc.ConfirmPasswordChange(ctx, req.GetChangeToken(), req.GetPassword(), req.GetPasswordConfirm()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.Empty{}, nil
}

func tokenPairResponse(tp *service.TokenPair) *authv1.TokenPairResponse {
	return &authv1.TokenPairResponse{
		AccessToken:  tp.AccessToken,
		RefreshToken: tp.RefreshToken,
	}
}

func profileResponse(p service.UserProfile) *authv1.UserProfileResponse {
	return &authv1.UserProfileResponse{
		Id:        p.ID,
		Email:     p.Email,
		FirstName: p.FirstName,
		LastName:  p.LastName,
	}
}

func mapError(err error) error {
	code := codes.Internal

	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		code = codes.Unauthenticated
	case errors.Is(err, service.ErrEmailAlreadyExists):
		code = codes.AlreadyExists
	case errors.Is(err, service.ErrUserNotFound):
		code = codes.NotFound

	case errors.Is(err, service.ErrNoFieldsToUpdate),
		errors.Is(err, service.ErrEmailRequired),
		errors.Is(err, service.ErrPasswordRequired),
		errors.Is(err, service.ErrInvalidEmailFormat),
		errors.Is(err, service.ErrInvalidPasswordRules),
		errors.Is(err, service.ErrPasswordMismatch),
		errors.Is(err, service.ErrResetCodeRequired),
		errors.Is(err, service.ErrResetCodeInvalid),
		errors.Is(err, service.ErrResetCodeExpired),
		errors.Is(err, service.ErrResetCodeAttempts),
		errors.Is(err, service.ErrResetTokenRequired),
		errors.Is(err, service.ErrChangeCodeRequired),
		errors.Is(err, service.ErrChangeCodeInvalid),
		errors.Is(err, service.ErrChangeCodeAttempts),
		errors.Is(err, service.ErrChangeTokenRequired):
		code = codes.InvalidArgument

	case errors.Is(err, service.ErrResetSessionInvalid),
		errors.Is(err, service.ErrResetSessionExpired),
		errors.Is(err, service.ErrChangeSessionInvalid),
		errors.Is(err, service.ErrChangeSessionExpired):
		code = codes.NotFound

	case errors.Is(err, service.ErrMailerNotConfigured),
		errors.Is(err, service.ErrRedisNotConfigured):
		code = codes.Unavailable

	case errors.Is(err, service.ErrTooManyRequests):
		code = codes.ResourceExhausted
	}

	return status.Error(code, err.Error())
}
