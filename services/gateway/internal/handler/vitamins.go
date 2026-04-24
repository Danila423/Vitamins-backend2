package handler

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vitaminsv1 "vitamins-backend_2/gen/go/vitamins/v1"
	"vitamins-backend_2/services/gateway/internal/middleware"
)

type VitaminsHandler struct {
	client vitaminsv1.VitaminsServiceClient
}

func NewVitaminsHandler(client vitaminsv1.VitaminsServiceClient) *VitaminsHandler {
	return &VitaminsHandler{client: client}
}

type catalogItemJSON struct {
	ID                    int64   `json:"id"`
	Code                  *string `json:"code,omitempty"`
	DisplayName           string  `json:"display_name"`
	DefaultUnit           *string `json:"default_unit,omitempty"`
	InteractionText       *string `json:"interaction_text,omitempty"`
	CompatibilityText     *string `json:"compatibility_text,omitempty"`
	ContraindicationsText *string `json:"contraindications_text,omitempty"`
	DefaultCondition      *string `json:"default_condition,omitempty"`
}

type courseResponseJSON struct {
	StartDate string  `json:"start_date"`
	EndDate   *string `json:"end_date,omitempty"`
	Timezone  string  `json:"timezone"`
}

type scheduleResponseJSON struct {
	Type  string   `json:"type"`
	Days  []string `json:"days"`
	Times []string `json:"times"`
}

type notificationPreferencesJSON struct {
	IncludeDose              bool `json:"include_dose"`
	IncludeFrequency         bool `json:"include_frequency"`
	IncludeInteraction       bool `json:"include_interaction"`
	IncludeCompatibility     bool `json:"include_compatibility"`
	IncludeCondition         bool `json:"include_condition"`
	IncludeContraindications bool `json:"include_contraindications"`
}

type contentOverridesJSON struct {
	InteractionTextOverride       *string `json:"interaction_text_override,omitempty"`
	CompatibilityTextOverride     *string `json:"compatibility_text_override,omitempty"`
	ContraindicationsTextOverride *string `json:"contraindications_text_override,omitempty"`
}

type reminderResponseJSON struct {
	ID                      int64                       `json:"id"`
	CatalogID               *int64                      `json:"catalog_id,omitempty"`
	Name                    string                      `json:"name"`
	Form                    string                      `json:"form"`
	Dose                    string                      `json:"dose"`
	Condition               string                      `json:"condition"`
	Note                    string                      `json:"note"`
	IsActive                bool                        `json:"is_active"`
	Catalog                 *catalogItemJSON            `json:"catalog,omitempty"`
	Course                  courseResponseJSON          `json:"course"`
	Schedule                scheduleResponseJSON        `json:"schedule"`
	NotificationPreferences notificationPreferencesJSON `json:"notification_preferences"`
	ContentOverrides        contentOverridesJSON        `json:"content_overrides"`
}

type createReminderRequestJSON struct {
	CatalogID               *int64                           `json:"catalog_id"`
	Name                    *string                          `json:"name"`
	Form                    *string                          `json:"form"`
	Dose                    *string                          `json:"dose"`
	Condition               *string                          `json:"condition"`
	Note                    *string                          `json:"note"`
	Course                  courseInputJSON                  `json:"course"`
	Schedule                scheduleInputJSON                `json:"schedule"`
	NotificationPreferences notificationPreferencesInputJSON `json:"notification_preferences"`
	ContentOverrides        contentOverridesInputJSON        `json:"content_overrides"`
}

type updateReminderRequestJSON struct {
	Name                    *string                           `json:"name"`
	Form                    *string                           `json:"form"`
	Dose                    *string                           `json:"dose"`
	Condition               *string                           `json:"condition"`
	Note                    *string                           `json:"note"`
	Course                  *courseInputJSON                  `json:"course"`
	Schedule                *scheduleInputJSON                `json:"schedule"`
	NotificationPreferences *notificationPreferencesInputJSON `json:"notification_preferences"`
	ContentOverrides        *contentOverridesInputJSON        `json:"content_overrides"`
}

type courseInputJSON struct {
	StartDate    string  `json:"start_date"`
	EndDate      *string `json:"end_date"`
	DurationDays *int    `json:"duration_days"`
	Timezone     string  `json:"timezone"`
}

type scheduleInputJSON struct {
	Days  []string `json:"days"`
	Times []string `json:"times"`
}

type notificationPreferencesInputJSON struct {
	IncludeDose              *bool `json:"include_dose"`
	IncludeFrequency         *bool `json:"include_frequency"`
	IncludeInteraction       *bool `json:"include_interaction"`
	IncludeCompatibility     *bool `json:"include_compatibility"`
	IncludeCondition         *bool `json:"include_condition"`
	IncludeContraindications *bool `json:"include_contraindications"`
}

type contentOverridesInputJSON struct {
	InteractionTextOverride       *string `json:"interaction_text_override"`
	CompatibilityTextOverride     *string `json:"compatibility_text_override"`
	ContraindicationsTextOverride *string `json:"contraindications_text_override"`
}

func (h *VitaminsHandler) ListCatalog(c *gin.Context) {
	ctx := c.Request.Context()
	resp, err := h.client.ListCatalog(ctx, &vitaminsv1.ListCatalogRequest{})
	if err != nil {
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	items := make([]catalogItemJSON, 0, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		items = append(items, catalogItemFromProto(it))
	}
	c.JSON(200, items)
}

func (h *VitaminsHandler) CreateReminder(c *gin.Context) {
	var r createReminderRequestJSON
	if err := c.ShouldBindJSON(&r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	req := &vitaminsv1.CreateReminderRequest{
		UserId:                  userID,
		CatalogId:               r.CatalogID,
		Name:                    r.Name,
		Form:                    r.Form,
		Dose:                    r.Dose,
		Condition:               r.Condition,
		Note:                    r.Note,
		Course:                  courseInputToProto(&r.Course),
		Schedule:                scheduleInputToProto(&r.Schedule),
		NotificationPreferences: notifPrefsInputToProto(&r.NotificationPreferences),
		ContentOverrides:        contentOverridesInputToProto(&r.ContentOverrides),
	}
	resp, err := h.client.CreateReminder(ctx, req)
	if err != nil {
		handleVitaminsError(c, err)
		return
	}
	logAudit(c).InfoContext(ctx, "reminder created",
		"operation", "vitamins.reminder.create",
		"user_id", userID,
		"reminder_id", resp.GetId(),
	)
	c.JSON(200, reminderFromProto(resp))
}

func (h *VitaminsHandler) ListReminders(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	ctx := c.Request.Context()
	resp, err := h.client.ListReminders(ctx, &vitaminsv1.ListRemindersRequest{UserId: userID})
	if err != nil {
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	reminders := make([]reminderResponseJSON, 0, len(resp.GetReminders()))
	for _, r := range resp.GetReminders() {
		reminders = append(reminders, reminderFromProto(r))
	}
	c.JSON(200, reminders)
}

func (h *VitaminsHandler) GetReminder(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.client.GetReminder(ctx, &vitaminsv1.GetReminderRequest{
		UserId:     userID,
		ReminderId: id,
	})
	if err != nil {
		handleVitaminsError(c, err)
		return
	}
	c.JSON(200, reminderFromProto(resp))
}

func (h *VitaminsHandler) UpdateReminder(c *gin.Context) {
	var body json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	var r updateReminderRequestJSON
	if err := json.Unmarshal(body, &r); err != nil {
		send(c, 400, "BAD_REQUEST", "Неверный формат запроса")
		return
	}
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	req := &vitaminsv1.UpdateReminderRequest{
		UserId:     userID,
		ReminderId: id,
		Name:       r.Name,
		Form:       r.Form,
		Dose:       r.Dose,
		Condition:  r.Condition,
		Note:       r.Note,
	}
	if r.Course != nil {
		req.Course = courseInputToProto(r.Course)
	}
	if r.Schedule != nil {
		req.Schedule = scheduleInputToProto(r.Schedule)
	}
	if r.NotificationPreferences != nil {
		req.NotificationPreferences = notifPrefsInputToProto(r.NotificationPreferences)
	}
	if r.ContentOverrides != nil {
		req.ContentOverrides = contentOverridesInputToProto(r.ContentOverrides)
	}
	resp, err := h.client.UpdateReminder(ctx, req)
	if err != nil {
		handleVitaminsError(c, err)
		return
	}
	logAudit(c).InfoContext(ctx, "reminder updated",
		"operation", "vitamins.reminder.update",
		"user_id", userID,
		"reminder_id", resp.GetId(),
	)
	c.JSON(200, reminderFromProto(resp))
}

func (h *VitaminsHandler) DeleteReminder(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.client.SetReminderActive(ctx, &vitaminsv1.SetReminderActiveRequest{
		UserId:     userID,
		ReminderId: id,
		Active:     false,
	})
	if err != nil {
		handleVitaminsError(c, err)
		return
	}
	logAudit(c).InfoContext(ctx, "reminder deleted",
		"operation", "vitamins.reminder.delete",
		"user_id", userID,
		"reminder_id", resp.GetId(),
	)
	c.JSON(200, reminderFromProto(resp))
}

func (h *VitaminsHandler) EnableReminder(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.client.SetReminderActive(ctx, &vitaminsv1.SetReminderActiveRequest{
		UserId:     userID,
		ReminderId: id,
		Active:     true,
	})
	if err != nil {
		handleVitaminsError(c, err)
		return
	}
	logAudit(c).InfoContext(ctx, "reminder enabled",
		"operation", "vitamins.reminder.enable",
		"user_id", userID,
		"reminder_id", resp.GetId(),
	)
	c.JSON(200, reminderFromProto(resp))
}

func (h *VitaminsHandler) DisableReminder(c *gin.Context) {
	userID, ok := middleware.UserIDFromContext(c)
	if !ok {
		send(c, 401, "AUTH_REQUIRED", "Требуется авторизация")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	resp, err := h.client.SetReminderActive(ctx, &vitaminsv1.SetReminderActiveRequest{
		UserId:     userID,
		ReminderId: id,
		Active:     false,
	})
	if err != nil {
		handleVitaminsError(c, err)
		return
	}
	logAudit(c).InfoContext(ctx, "reminder disabled",
		"operation", "vitamins.reminder.disable",
		"user_id", userID,
		"reminder_id", resp.GetId(),
	)
	c.JSON(200, reminderFromProto(resp))
}

func handleVitaminsError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		logApp(c).ErrorContext(c.Request.Context(), "vitamins handler failed",
			"operation", "vitamins.handler.error", "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
		return
	}
	msg := st.Message()
	switch st.Code() {
	case codes.InvalidArgument:
		switch {
		case strings.Contains(msg, "NAME_REQUIRED"):
			send(c, 400, "NAME_REQUIRED", "Введите название витамина")
		case strings.Contains(msg, "INVALID_FORM"):
			send(c, 400, "INVALID_FORM", "Неверная форма препарата")
		case strings.Contains(msg, "INVALID_CONDITION"):
			send(c, 400, "INVALID_CONDITION", "Неверное условие приема")
		case strings.Contains(msg, "INVALID_DAYS"):
			send(c, 400, "INVALID_DAYS", "Неверный список дней")
		case strings.Contains(msg, "INVALID_TIMES"):
			send(c, 400, "INVALID_TIMES", "Неверное время приема")
		case strings.Contains(msg, "START_DATE_REQUIRED"):
			send(c, 400, "START_DATE_REQUIRED", "Введите дату начала")
		case strings.Contains(msg, "INVALID_DATE"):
			send(c, 400, "INVALID_DATE_FORMAT", "Неверный формат даты")
		case strings.Contains(msg, "INVALID_COURSE_DURATION"):
			send(c, 400, "INVALID_COURSE_DURATION", "Неверная длительность курса")
		case strings.Contains(msg, "TIMEZONE_REQUIRED"):
			send(c, 400, "TIMEZONE_REQUIRED", "Укажите часовой пояс")
		case strings.Contains(msg, "NO_FIELDS_TO_UPDATE"):
			send(c, 400, "NO_FIELDS_TO_UPDATE", "Нечего обновлять")
		case strings.Contains(msg, "INVALID_ID"):
			send(c, 400, "INVALID_ID", "Неверный идентификатор")
		default:
			send(c, 400, "BAD_REQUEST", msg)
		}
	case codes.NotFound:
		switch {
		case strings.Contains(msg, "CATALOG_NOT_FOUND"):
			send(c, 404, "CATALOG_NOT_FOUND", "Витамин не найден")
		default:
			send(c, 404, "REMINDER_NOT_FOUND", "Напоминание не найдено")
		}
	default:
		logApp(c).ErrorContext(c.Request.Context(), "vitamins handler failed",
			"operation", "vitamins.handler.error", "error", err.Error())
		send(c, 500, "INTERNAL_ERROR", "Что-то пошло не так.")
	}
}

func parseID(c *gin.Context, name string) (int64, bool) {
	raw := c.Param(name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		send(c, 400, "INVALID_ID", "Неверный идентификатор")
		return 0, false
	}
	return id, true
}

func catalogItemFromProto(pb *vitaminsv1.CatalogItem) catalogItemJSON {
	if pb == nil {
		return catalogItemJSON{}
	}
	return catalogItemJSON{
		ID:                    pb.GetId(),
		Code:                  pb.Code,
		DisplayName:           pb.GetDisplayName(),
		DefaultUnit:           pb.DefaultUnit,
		InteractionText:       pb.InteractionText,
		CompatibilityText:     pb.CompatibilityText,
		ContraindicationsText: pb.ContraindicationsText,
		DefaultCondition:      pb.DefaultCondition,
	}
}

func reminderFromProto(pb *vitaminsv1.ReminderResponse) reminderResponseJSON {
	if pb == nil {
		return reminderResponseJSON{}
	}
	r := reminderResponseJSON{
		ID:        pb.GetId(),
		CatalogID: pb.CatalogId,
		Name:      pb.GetName(),
		Form:      pb.GetForm(),
		Dose:      pb.GetDose(),
		Condition: pb.GetCondition(),
		Note:      pb.GetNote(),
		IsActive:  pb.GetIsActive(),
	}
	if pb.Catalog != nil {
		cat := catalogItemFromProto(pb.GetCatalog())
		r.Catalog = &cat
	}
	if c := pb.GetCourse(); c != nil {
		r.Course = courseResponseJSON{
			StartDate: c.GetStartDate(),
			EndDate:   c.EndDate,
			Timezone:  c.GetTimezone(),
		}
	}
	if s := pb.GetSchedule(); s != nil {
		r.Schedule = scheduleResponseJSON{
			Type:  s.GetType(),
			Days:  s.GetDays(),
			Times: s.GetTimes(),
		}
	}
	if n := pb.GetNotificationPreferences(); n != nil {
		r.NotificationPreferences = notificationPreferencesJSON{
			IncludeDose:              n.GetIncludeDose(),
			IncludeFrequency:         n.GetIncludeFrequency(),
			IncludeInteraction:       n.GetIncludeInteraction(),
			IncludeCompatibility:     n.GetIncludeCompatibility(),
			IncludeCondition:         n.GetIncludeCondition(),
			IncludeContraindications: n.GetIncludeContraindications(),
		}
	}
	if co := pb.GetContentOverrides(); co != nil {
		r.ContentOverrides = contentOverridesJSON{
			InteractionTextOverride:       co.InteractionTextOverride,
			CompatibilityTextOverride:     co.CompatibilityTextOverride,
			ContraindicationsTextOverride: co.ContraindicationsTextOverride,
		}
	}
	return r
}

func courseInputToProto(c *courseInputJSON) *vitaminsv1.CourseInput {
	if c == nil {
		return nil
	}
	p := &vitaminsv1.CourseInput{
		StartDate: c.StartDate,
		EndDate:   c.EndDate,
		Timezone:  c.Timezone,
	}
	if c.DurationDays != nil {
		d := safeVitInt32(*c.DurationDays)
		p.DurationDays = &d
	}
	return p
}

func scheduleInputToProto(s *scheduleInputJSON) *vitaminsv1.ScheduleInput {
	if s == nil {
		return nil
	}
	return &vitaminsv1.ScheduleInput{
		Days:  s.Days,
		Times: s.Times,
	}
}

func notifPrefsInputToProto(n *notificationPreferencesInputJSON) *vitaminsv1.NotificationPreferencesInput {
	if n == nil {
		return nil
	}
	return &vitaminsv1.NotificationPreferencesInput{
		IncludeDose:              n.IncludeDose,
		IncludeFrequency:         n.IncludeFrequency,
		IncludeInteraction:       n.IncludeInteraction,
		IncludeCompatibility:     n.IncludeCompatibility,
		IncludeCondition:         n.IncludeCondition,
		IncludeContraindications: n.IncludeContraindications,
	}
}

func contentOverridesInputToProto(co *contentOverridesInputJSON) *vitaminsv1.ContentOverridesInput {
	if co == nil {
		return nil
	}
	return &vitaminsv1.ContentOverridesInput{
		InteractionTextOverride:       co.InteractionTextOverride,
		CompatibilityTextOverride:     co.CompatibilityTextOverride,
		ContraindicationsTextOverride: co.ContraindicationsTextOverride,
	}
}

func safeVitInt32(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
