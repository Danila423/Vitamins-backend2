package grpc

import (
	"context"
	"errors"

	vitaminsv1 "vitamins-backend_2/gen/go/vitamins/v1"
	"vitamins-backend_2/services/vitamins/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	vitaminsv1.UnimplementedVitaminsServiceServer
	svc service.ServiceAPI
}

func NewServer(svc service.ServiceAPI) *Server {
	return &Server{svc: svc}
}

func (s *Server) ListCatalog(ctx context.Context, _ *vitaminsv1.ListCatalogRequest) (*vitaminsv1.ListCatalogResponse, error) {
	items, err := s.svc.ListCatalog(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	pbItems := make([]*vitaminsv1.CatalogItem, len(items))
	for i := range items {
		pbItems[i] = catalogItemToProto(&items[i])
	}
	return &vitaminsv1.ListCatalogResponse{Items: pbItems}, nil
}

func (s *Server) CreateReminder(ctx context.Context, req *vitaminsv1.CreateReminderRequest) (*vitaminsv1.ReminderResponse, error) {
	domReq := protoToCreateRequest(req)
	resp, err := s.svc.CreateReminder(ctx, req.GetUserId(), domReq)
	if err != nil {
		return nil, mapError(err)
	}
	return reminderResponseToProto(&resp), nil
}

func (s *Server) ListReminders(ctx context.Context, req *vitaminsv1.ListRemindersRequest) (*vitaminsv1.ListRemindersResponse, error) {
	reminders, err := s.svc.ListReminders(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	pbReminders := make([]*vitaminsv1.ReminderResponse, len(reminders))
	for i := range reminders {
		pbReminders[i] = reminderResponseToProto(&reminders[i])
	}
	return &vitaminsv1.ListRemindersResponse{Reminders: pbReminders}, nil
}

func (s *Server) GetReminder(ctx context.Context, req *vitaminsv1.GetReminderRequest) (*vitaminsv1.ReminderResponse, error) {
	resp, err := s.svc.GetReminder(ctx, req.GetUserId(), req.GetReminderId())
	if err != nil {
		return nil, mapError(err)
	}
	return reminderResponseToProto(&resp), nil
}

func (s *Server) UpdateReminder(ctx context.Context, req *vitaminsv1.UpdateReminderRequest) (*vitaminsv1.ReminderResponse, error) {
	domReq := protoToUpdateRequest(req)
	resp, err := s.svc.UpdateReminder(ctx, req.GetUserId(), req.GetReminderId(), domReq)
	if err != nil {
		return nil, mapError(err)
	}
	return reminderResponseToProto(&resp), nil
}

func (s *Server) SetReminderActive(ctx context.Context, req *vitaminsv1.SetReminderActiveRequest) (*vitaminsv1.ReminderResponse, error) {
	resp, err := s.svc.SetReminderActive(ctx, req.GetUserId(), req.GetReminderId(), req.GetActive())
	if err != nil {
		return nil, mapError(err)
	}
	return reminderResponseToProto(&resp), nil
}

func catalogItemToProto(item *service.CatalogItem) *vitaminsv1.CatalogItem {
	return &vitaminsv1.CatalogItem{
		Id:                    item.ID,
		Code:                  item.Code,
		DisplayName:           item.DisplayName,
		DefaultUnit:           item.DefaultUnit,
		InteractionText:       item.InteractionText,
		CompatibilityText:     item.CompatibilityText,
		ContraindicationsText: item.ContraindicationsText,
		DefaultCondition:      item.DefaultCondition,
	}
}

func reminderResponseToProto(r *service.ReminderResponse) *vitaminsv1.ReminderResponse {
	pb := &vitaminsv1.ReminderResponse{
		Id:        r.ID,
		CatalogId: r.CatalogID,
		Name:      r.Name,
		Form:      r.Form,
		Dose:      r.Dose,
		Condition: r.Condition,
		Note:      r.Note,
		IsActive:  r.IsActive,
		Course: &vitaminsv1.CourseResponse{
			StartDate: r.Course.StartDate,
			EndDate:   r.Course.EndDate,
			Timezone:  r.Course.Timezone,
		},
		Schedule: &vitaminsv1.ScheduleResponse{
			Type:  r.Schedule.Type,
			Days:  r.Schedule.Days,
			Times: r.Schedule.Times,
		},
		NotificationPreferences: &vitaminsv1.NotificationPreferencesResponse{
			IncludeDose:              r.NotificationPreferences.IncludeDose,
			IncludeFrequency:         r.NotificationPreferences.IncludeFrequency,
			IncludeInteraction:       r.NotificationPreferences.IncludeInteraction,
			IncludeCompatibility:     r.NotificationPreferences.IncludeCompatibility,
			IncludeCondition:         r.NotificationPreferences.IncludeCondition,
			IncludeContraindications: r.NotificationPreferences.IncludeContraindications,
		},
		ContentOverrides: &vitaminsv1.ContentOverridesResponse{
			InteractionTextOverride:       r.ContentOverrides.InteractionTextOverride,
			CompatibilityTextOverride:     r.ContentOverrides.CompatibilityTextOverride,
			ContraindicationsTextOverride: r.ContentOverrides.ContraindicationsTextOverride,
		},
	}
	if r.Catalog != nil {
		pb.Catalog = catalogItemToProto(r.Catalog)
	}
	return pb
}

func protoToCreateRequest(req *vitaminsv1.CreateReminderRequest) service.CreateReminderRequest {
	domReq := service.CreateReminderRequest{
		CatalogID: req.CatalogId,
		Name:      req.Name,
		Form:      req.Form,
		Dose:      req.Dose,
		Condition: req.Condition,
		Note:      req.Note,
	}
	if c := req.GetCourse(); c != nil {
		domReq.Course = service.CourseInput{
			StartDate: c.GetStartDate(),
			EndDate:   c.EndDate,
			Timezone:  c.GetTimezone(),
		}
		if c.DurationDays != nil {
			d := int(*c.DurationDays)
			domReq.Course.DurationDays = &d
		}
	}
	if s := req.GetSchedule(); s != nil {
		domReq.Schedule = service.ScheduleInput{
			Days:  s.GetDays(),
			Times: s.GetTimes(),
		}
	}
	if np := req.GetNotificationPreferences(); np != nil {
		domReq.NotificationPreferences = service.NotificationPreferencesInput{
			IncludeDose:              np.IncludeDose,
			IncludeFrequency:         np.IncludeFrequency,
			IncludeInteraction:       np.IncludeInteraction,
			IncludeCompatibility:     np.IncludeCompatibility,
			IncludeCondition:         np.IncludeCondition,
			IncludeContraindications: np.IncludeContraindications,
		}
	}
	if co := req.GetContentOverrides(); co != nil {
		domReq.ContentOverrides = service.ContentOverridesInput{
			InteractionTextOverride:       co.InteractionTextOverride,
			CompatibilityTextOverride:     co.CompatibilityTextOverride,
			ContraindicationsTextOverride: co.ContraindicationsTextOverride,
		}
	}
	return domReq
}

func protoToUpdateRequest(req *vitaminsv1.UpdateReminderRequest) service.UpdateReminderRequest {
	domReq := service.UpdateReminderRequest{
		Name:      req.Name,
		Form:      req.Form,
		Dose:      req.Dose,
		Condition: req.Condition,
		Note:      req.Note,
	}
	if req.Course != nil {
		c := req.GetCourse()
		ci := service.CourseInput{
			StartDate: c.GetStartDate(),
			EndDate:   c.EndDate,
			Timezone:  c.GetTimezone(),
		}
		if c.DurationDays != nil {
			d := int(*c.DurationDays)
			ci.DurationDays = &d
		}
		domReq.Course = &ci
	}
	if req.Schedule != nil {
		s := req.GetSchedule()
		si := service.ScheduleInput{
			Days:  s.GetDays(),
			Times: s.GetTimes(),
		}
		domReq.Schedule = &si
	}
	if req.NotificationPreferences != nil {
		np := req.GetNotificationPreferences()
		npi := service.NotificationPreferencesInput{
			IncludeDose:              np.IncludeDose,
			IncludeFrequency:         np.IncludeFrequency,
			IncludeInteraction:       np.IncludeInteraction,
			IncludeCompatibility:     np.IncludeCompatibility,
			IncludeCondition:         np.IncludeCondition,
			IncludeContraindications: np.IncludeContraindications,
		}
		domReq.NotificationPreferences = &npi
	}
	if req.ContentOverrides != nil {
		co := req.GetContentOverrides()
		coi := service.ContentOverridesInput{
			InteractionTextOverride:       co.InteractionTextOverride,
			CompatibilityTextOverride:     co.CompatibilityTextOverride,
			ContraindicationsTextOverride: co.ContraindicationsTextOverride,
		}
		domReq.ContentOverrides = &coi
	}
	return domReq
}

func mapError(err error) error {
	switch {
	case errors.Is(err, service.ErrCatalogNotFound), errors.Is(err, service.ErrReminderNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrNameRequired),
		errors.Is(err, service.ErrInvalidForm),
		errors.Is(err, service.ErrInvalidCondition),
		errors.Is(err, service.ErrInvalidDays),
		errors.Is(err, service.ErrInvalidTimes),
		errors.Is(err, service.ErrStartDateRequired),
		errors.Is(err, service.ErrInvalidDate),
		errors.Is(err, service.ErrInvalidCourseDuration),
		errors.Is(err, service.ErrTimezoneRequired),
		errors.Is(err, service.ErrNoFieldsToUpdate):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
