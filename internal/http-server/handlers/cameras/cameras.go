package camerashandler

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
	"github.com/zanzhit/studio_recorder/internal/http-server/handlers"
	"github.com/zanzhit/studio_recorder/internal/lib/api/response"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type CameraHandler struct {
	log      *slog.Logger
	recorder Recorder
	camera   Camera
}

type Recorder interface {
	Start(cameraIP []string, userID int) (string, error)
	Stop(cameraIP []string, userID int) (string, error)
	Schedule(rec models.ScheduleRecording, userID int) error
}

type Camera interface {
	SaveCamera(models.Camera) (models.Camera, error)
}

func New(
	log *slog.Logger,
	recorder Recorder,
	camera Camera,
) *CameraHandler {
	return &CameraHandler{
		log:      log,
		recorder: recorder,
		camera:   camera,
	}
}

func (h *CameraHandler) SaveCamera(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.SaveCamera"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req models.Camera
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

	cam, err := h.camera.SaveCamera(req)
	if err != nil {
		if errors.Is(err, errs.ErrCameraAlreadyExists) {
			handlers.Error(w, r, http.StatusBadRequest, response.Error("camera already exists", ""))

			return
		}

		log.Error("failed to save new camera", sl.Err(err))

		handlers.Error(w, r, http.StatusInternalServerError, response.Error("failed to save new camera", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, cam)
}
