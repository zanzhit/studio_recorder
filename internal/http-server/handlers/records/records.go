package recordshandler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/http-server/handlers"
	authmiddleware "github.com/zanzhit/studio_recorder/internal/http-server/middleware/auth"
	"github.com/zanzhit/studio_recorder/internal/lib/api/response"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type RecordHandler struct {
	log            *slog.Logger
	recordProvider RecordProvider
	recorder       Recorder
}

type RecordProvider interface {
	Record(camera string, limit, userID int) ([]models.Recording, error)
	Move(recordID string) error
}

type Recorder interface {
	Start(cameraIP []string, userID int) (string, error)
	Stop(recordId string, userID int) (string, error)
	Schedule(rec models.ScheduleRecording, userID int) error
}

func New(log *slog.Logger, recordProvider RecordProvider, recorder Recorder) *RecordHandler {
	return &RecordHandler{
		log:            log,
		recordProvider: recordProvider,
		recorder:       recorder,
	}
}

type RequestIP struct {
	CameraIPs []string `json:"camera_ips" validate:"required"`
}

type RequestID struct {
	RecordID string `json:"record_id" validate:"required"`
}

func (h *RecordHandler) Record(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.records.Record"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	cameraIP := r.URL.Query().Get("cameras")
	if cameraIP == "" {
		log.Error("cameraIP is empty")

		handlers.Error(w, r, http.StatusBadRequest, response.Error("missing cameraIP query parameter", ""))

		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 1

	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			log.Error("invalid limit parameter", sl.Err(err))

			handlers.Error(w, r, http.StatusBadRequest, response.Error("invalid limit parameter", ""))

			return
		}
	}

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		handlers.Error(w, r, http.StatusUnauthorized, response.Error("user not found", ""))

		return
	}

	rec, err := h.recordProvider.Record(cameraIP, limit, user.Id)
	if err != nil {
		log.Error("failed to ger recording", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to get recording", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, rec)
}

func (h *RecordHandler) Move(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.records.Move"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req RequestID
	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			handlers.Error(w, r, http.StatusBadRequest, response.Error("empty request", ""))

			return
		}

		log.Error("failed to decode request body", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to decode request", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("request body decoded", slog.Any("request", req))

	if err := validator.New().Struct(req); err != nil {
		validateErr := err.(validator.ValidationErrors)

		log.Error("invalid request", sl.Err(err))

		handlers.Error(w, r, http.StatusBadRequest, response.ValidationError(validateErr))

		return
	}

	if err = h.recordProvider.Move(req.RecordID); err != nil {
		log.Error("failed to move recording", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to move recording", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *RecordHandler) Start(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Start"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req RequestIP
	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			handlers.Error(w, r, http.StatusBadRequest, response.Error("empty request", ""))

			return
		}

		log.Error("failed to decode request body", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to decode request", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("request body decoded", slog.Any("request", req))

	if err := validator.New().Struct(req); err != nil {
		validateErr := err.(validator.ValidationErrors)

		log.Error("invalid request", sl.Err(err))

		handlers.Error(w, r, http.StatusBadRequest, response.ValidationError(validateErr))

		return
	}

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		handlers.Error(w, r, http.StatusUnauthorized, response.Error("user not found", ""))

		return
	}

	recordID, err := h.recorder.Start(req.CameraIPs, user.Id)
	if err != nil {
		log.Error("failed to start recording", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to start recording", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, map[string]string{"id": recordID})
}

func (h *RecordHandler) Stop(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Stop"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req RequestID
	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			handlers.Error(w, r, http.StatusBadRequest, response.Error("empty request", ""))

			return
		}

		log.Error("failed to decode request body", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to decode request", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("request body decoded", slog.Any("request", req))

	if err := validator.New().Struct(req); err != nil {
		validateErr := err.(validator.ValidationErrors)

		log.Error("invalid request", sl.Err(err))

		handlers.Error(w, r, http.StatusBadRequest, response.ValidationError(validateErr))

		return
	}

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		handlers.Error(w, r, http.StatusUnauthorized, response.Error("user not found", ""))

		return
	}

	recordID, err := h.recorder.Stop(req.RecordID, user.Id)
	if err != nil {
		log.Error("failed to stop recording", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to stop recording", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, map[string]string{"record id": recordID})
}

func (h *RecordHandler) Schedule(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Schedule"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var rec models.ScheduleRecording

	err := render.DecodeJSON(r.Body, &rec)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			handlers.Error(w, r, http.StatusBadRequest, response.Error("empty request", ""))

			return
		}

		log.Error("failed to decode request body", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to decode request", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("request body decoded", slog.Any("request", rec))

	if err := validator.New().Struct(rec); err != nil {
		validateErr := err.(validator.ValidationErrors)

		log.Error("invalid request", sl.Err(err))

		handlers.Error(w, r, http.StatusBadRequest, response.ValidationError(validateErr))

		return
	}

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		handlers.Error(w, r, http.StatusUnauthorized, response.Error("user not found", ""))

		return
	}

	if err := h.recorder.Schedule(rec, user.Id); err != nil {
		log.Error("failed to schedule recording", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to schedule recording", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}
