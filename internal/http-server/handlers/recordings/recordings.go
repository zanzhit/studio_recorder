package recordinghandler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

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
	CameraRecordings(camera string, limit, userID int) ([]models.Recording, error)
	Move(recordID string) error
}

type Recorder interface {
	Start(cameraIP []string, userID int) (string, error)
	Stop(recordId string) error
	Schedule(rec models.ScheduleRecording, userID int) error
}

func New(log *slog.Logger, recordingProvider RecordingProvider, recorder Recorder) *RecordHandler {
	return &RecordHandler{
		log:               log,
		recordingProvider: recordingProvider,
		recorder:          recorder,
	}
}

type RequestIP struct {
	CameraIPs []string `json:"camera_ips" validate:"required"`
}

type RequestID struct {
	RecordID string `json:"record_id" validate:"required"`
}

type Response struct {
	RecordID string `json:"record_id,omitempty"`
	response.Response
}

func (h *RecordHandler) Recordings(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.records.Record"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req RequestIP
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

	if len(req.CameraIPs) != 1 {
		log.Error("invalid request", sl.Err(err))

		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, response.Error("too many ips", middleware.GetReqID(r.Context())))

		return
	}

	log.Info("request body decoded", slog.Any("request", req))

	limit := 5

	user, ok := r.Context().Value(authmiddleware.UserContextKey).(models.User)
	if !ok {
		log.Error("user not found in context")

		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, response.Error("user not found", ""))

		return
	}

	rec, err := h.recordingProvider.CameraRecordings(req.CameraIPs[0], limit, user.Id)
	if err != nil {
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

	var req RequestID
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

	if err = h.recordingProvider.Move(req.RecordID); err != nil {
		if errors.Is(err, errs.ErrWriteToDB) {
			response := Response{
				Response: response.Response{
					Error: "failed to write move data",
				},
			}

			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response)

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

	var req RequestIP
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

	recordID, err := h.recorder.Start(req.CameraIPs, user.Id)
	if err != nil {
		if errors.Is(err, errs.ErrWriteToDB) {
			response := Response{
				RecordID: recordID,
				Response: response.Response{
					Error: "failed to write start data",
				},
			}

			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response)

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

	var req RequestID
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

	if err = h.recorder.Stop(req.RecordID); err != nil {
		if errors.Is(err, errs.ErrWriteToDB) {
			response := Response{
				Response: response.Response{
					Error: "failed to write start data",
				},
			}

			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, response)

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

	var rec models.ScheduleRecording

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

	if err := h.recorder.Schedule(rec, user.Id); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to schedule recording", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}
