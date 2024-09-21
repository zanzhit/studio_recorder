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
	"github.com/zanzhit/studio_recorder/internal/lib/api/response"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type CameraHandler struct {
	log    *slog.Logger
	camera Camera
}

type Camera interface {
	SaveCamera(models.Camera) (models.Camera, error)
}

func New(
	log *slog.Logger,
	camera Camera,
) *CameraHandler {
	return &CameraHandler{
		log:    log,
		camera: camera,
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

	cam, err := h.camera.SaveCamera(req)
	if err != nil {
		if errors.Is(err, errs.ErrCameraAlreadyExists) {
			log.Error("camera is already exist", sl.Err(err))

			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("camera already exists", ""))

			return
		}

		log.Error("failed to save camera", sl.Err(err))

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to save new camera", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, cam)
}
