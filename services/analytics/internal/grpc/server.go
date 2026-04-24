package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"math"

	analyticsv1 "vitamins-backend_2/gen/go/analytics/v1"
	"vitamins-backend_2/services/analytics/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	analyticsv1.UnimplementedAnalyticsServiceServer
	svc service.ServiceAPI
}

func NewServer(svc service.ServiceAPI) *Server {
	return &Server{svc: svc}
}

func (s *Server) Ingest(ctx context.Context, req *analyticsv1.IngestRequest) (*analyticsv1.IngestResponse, error) {
	var tokenUserID *int64
	if req.UserId != nil {
		v := req.GetUserId()
		tokenUserID = &v
	}

	batch := service.BatchRequest{}
	if b := req.GetBatch(); b != nil {
		batch.BatchID = b.BatchId
		batch.SentAt = b.SentAt
		events := make([]service.EventInput, 0, len(b.GetEvents()))
		for _, e := range b.GetEvents() {
			props := map[string]any{}
			if len(e.GetPropertiesJson()) > 0 {
				if err := json.Unmarshal(e.GetPropertiesJson(), &props); err != nil {
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
			}
			events = append(events, service.EventInput{
				EventID:     e.GetEventId(),
				OccurredAt:  e.GetOccurredAt(),
				EventName:   e.GetEventName(),
				SessionID:   e.GetSessionId(),
				UserID:      e.UserId,
				AnonymousID: e.AnonymousId,
				Properties:  props,
				RequestID:   e.RequestId,
				AppVersion:  e.AppVersion,
				Platform:    e.Platform,
			})
		}
		batch.Events = events
	}

	resp, err := s.svc.Ingest(ctx, tokenUserID, batch)
	if err != nil {
		return nil, mapError(err)
	}

	return &analyticsv1.IngestResponse{
		Accepted:     safeInt32(resp.Accepted),
		Deduplicated: safeInt32(resp.Deduplicated),
	}, nil
}

func (s *Server) SetConsent(ctx context.Context, req *analyticsv1.SetConsentRequest) (*analyticsv1.SetConsentResponse, error) {
	if err := s.svc.SetConsent(ctx, req.GetUserId(), req.GetConsent()); err != nil {
		return nil, mapError(err)
	}
	return &analyticsv1.SetConsentResponse{}, nil
}

func (s *Server) GetConsent(ctx context.Context, req *analyticsv1.GetConsentRequest) (*analyticsv1.GetConsentResponse, error) {
	consent, err := s.svc.GetConsent(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return &analyticsv1.GetConsentResponse{Consent: consent}, nil
}

func (s *Server) Export(ctx context.Context, req *analyticsv1.ExportRequest) (*analyticsv1.ExportResponse, error) {
	filter := service.ExportFilter{
		From:      req.From,
		To:        req.To,
		EventName: req.EventName,
		UserID:    req.UserId,
		Limit:     int(req.GetLimit()),
		Offset:    int(req.GetOffset()),
	}

	rows, err := s.svc.Export(ctx, filter)
	if err != nil {
		return nil, mapError(err)
	}

	pbRows := make([]*analyticsv1.ExportRow, 0, len(rows))
	for _, r := range rows {
		pbRows = append(pbRows, &analyticsv1.ExportRow{
			EventId:     r.EventID,
			OccurredAt:  r.OccurredAt,
			ReceivedAt:  r.ReceivedAt,
			UserId:      r.UserID,
			AnonymousId: r.AnonymousID,
			SessionId:   r.SessionID,
			EventName:   r.EventName,
			Properties:  r.Properties,
			RequestId:   r.RequestID,
			AppVersion:  r.AppVersion,
			Platform:    r.Platform,
		})
	}

	return &analyticsv1.ExportResponse{Rows: pbRows}, nil
}

func safeInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

func mapError(err error) error {
	switch {
	case errors.Is(err, service.ErrEmptyBatch),
		errors.Is(err, service.ErrBatchTooLarge),
		errors.Is(err, service.ErrInvalidEventID),
		errors.Is(err, service.ErrInvalidOccurredAt),
		errors.Is(err, service.ErrInvalidEventName),
		errors.Is(err, service.ErrInvalidSessionID),
		errors.Is(err, service.ErrInvalidAnonymousID),
		errors.Is(err, service.ErrAnonymousRequired):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrConsentRequired):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
