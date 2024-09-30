package recordinghandler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"

	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	authmiddleware "github.com/zanzhit/studio_recorder/internal/http-server/middleware/auth"
	"github.com/zanzhit/studio_recorder/internal/lib/api/response"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type RecordHandler struct {
	log               *slog.Logger
	recordingProvider RecordingProvider
	recorder          Recorder
}

type RecordingProvider interface {
	CameraRecordings(camera string, limit, offset, userID int) ([]models.Recording, error)
	Delete(recordID string) error
	Move(recordID string) error
	File(recordID string) (string, error)
}

type Recorder interface {
	Start(cameraID []string, userID int) (string, error)
	Stop(recordId string) error
	Schedule(startTime time.Time, cameraID []string, duration string, userID int) error
}

func New(log *slog.Logger, recordingProvider RecordingProvider, recorder Recorder) *RecordHandler {
	return &RecordHandler{
		log:               log,
		recordingProvider: recordingProvider,
		recorder:          recorder,
	}
}

type RequestStart struct {
	CameraIDs []string `json:"camera_ids" validate:"required"`
}

type RequestSchedule struct {
	CameraID  []string  `json:"camera_id" validate:"required"`
	Duration  string    `json:"duration" validate:"required"`
	StartTime time.Time `json:"start_time" validate:"required"`
}

type Response struct {
	RecordID string `json:"record_id"`
	response.Response
}

func (h *RecordHandler) Recordings(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.records.Record"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	cameraID := chi.URLParam(r, "cameraID")
	if cameraID == "" {
		log.Error("camera_id is empty")

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.Error("camera_id is empty", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("camera_id", slog.String("camera_id", cameraID))

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 5
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, response.Error("user not found", ""))

		return
	}

	rec, err := h.recordingProvider.CameraRecordings(cameraID, limit, offset, user.Id)
	if err != nil {
		if errors.Is(err, errs.ErrRecordNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("recording not found", middleware.GetReqID(r.Context())))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to get recording", middleware.GetReqID(r.Context())))

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

	recordID := chi.URLParam(r, "recordID")
	if recordID == "" {
		log.Error("record_id is empty")

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.Error("record_id is empty", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("record_id", slog.String("record_id", recordID))

	if err := h.recordingProvider.Move(recordID); err != nil {
		if errors.Is(err, errs.ErrRecordNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("recording not found", middleware.GetReqID(r.Context())))

			return
		}

		if errors.Is(err, errs.ErrWriteToDB) {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("recording moved, but failed to write move data", middleware.GetReqID(r.Context())))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to move recording", middleware.GetReqID(r.Context())))

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

	var req RequestStart
	err := render.DecodeJSON(r.Body, &req)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("empty request", ""))

			return
		}

		log.Error("failed to decode request body", sl.Err(err))

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to decode request", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("request body decoded", slog.Any("request", req))

	if err := validator.New().Struct(req); err != nil {
		validateErr := err.(validator.ValidationErrors)

		log.Error("invalid request", sl.Err(err))

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.ValidationError(validateErr))

		return
	}

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, response.Error("user not found", ""))

		return
	}

	recordID, err := h.recorder.Start(req.CameraIDs, user.Id)
	if err != nil {
		if errors.Is(err, errs.ErrWriteToDB) {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, Response{RecordID: recordID, Response: response.Error("failed to write start data", middleware.GetReqID(r.Context()))})

			return
		}
		if errors.Is(err, errs.ErrCameraIsNotAvailable) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("camera is not available", middleware.GetReqID(r.Context())))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to start recording", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, Response{RecordID: recordID})
}

func (h *RecordHandler) Stop(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Stop"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	recordID := chi.URLParam(r, "recordID")
	if recordID == "" {
		log.Error("record_id is empty")

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.Error("record_id is empty", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("record_id", slog.String("record_id", recordID))

	if err := h.recorder.Stop(recordID); err != nil {
		if errors.Is(err, errs.ErrRecordNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("recording not found", middleware.GetReqID(r.Context())))

			return
		}

		if errors.Is(err, errs.ErrWriteToDB) {
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response.Error("recording stopped, but failed to write stop data", middleware.GetReqID(r.Context())))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to stop recording", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *RecordHandler) Schedule(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Schedule"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var rec RequestSchedule

	err := render.DecodeJSON(r.Body, &rec)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Error("request body is empty")

			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("empty request", ""))

			return
		}

		log.Error("failed to decode request body", sl.Err(err))

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to decode request", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("request body decoded", slog.Any("request", rec))

	if err := validator.New().Struct(rec); err != nil {
		validateErr := err.(validator.ValidationErrors)

		log.Error("invalid request", sl.Err(err))

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.ValidationError(validateErr))

		return
	}

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, response.Error("user not found", ""))

		return
	}

	if err := h.recorder.Schedule(rec.StartTime, rec.CameraID, rec.Duration, user.Id); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to schedule recording", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *RecordHandler) Delete(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Delete"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	recordID := chi.URLParam(r, "recordID")
	if recordID == "" {
		log.Error("record_id is empty")

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.Error("record_id is empty", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("record_id", slog.String("record_id", recordID))

	if err := h.recordingProvider.Delete(recordID); err != nil {
		if errors.Is(err, errs.ErrRecordNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("recording not found", middleware.GetReqID(r.Context())))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to delete recording", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *RecordHandler) Download(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Download"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	recordID := chi.URLParam(r, "recordID")
	if recordID == "" {
		log.Error("record_id is empty")

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.Error("record_id is empty", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("record_id", slog.String("record_id", recordID))

	filePath, err := h.recordingProvider.File(recordID)
	if err != nil {
		if errors.Is(err, errs.ErrRecordNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("recording not found", middleware.GetReqID(r.Context())))

			return
		}
		if errors.Is(err, errs.ErrFileNotFound) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("recording file not found", middleware.GetReqID(r.Context())))

			return
		}
		if errors.Is(err, errs.ErrFileAlreadyMoved) {
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("recording file already moved", middleware.GetReqID(r.Context())))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to get recording", middleware.GetReqID(r.Context())))

		return
	}

	http.ServeFile(w, r, filePath)
}
