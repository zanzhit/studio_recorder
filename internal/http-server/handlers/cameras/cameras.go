package camerashandler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"

	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/lib/api/response"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type CameraHandler struct {
	log            *slog.Logger
	cameraSaver    CameraSaver
	cameraProvider CameraProvider
}

type CameraSaver interface {
	SaveCamera(cameraIP, location string, hasAudio bool) (models.Camera, error)
}
type CameraProvider interface {
	Cameras() ([]models.Camera, error)
	UpdateCamera(cameraID, location string, hasAudio bool) (models.Camera, error)
	DeleteCamera(string) error
}

func New(
	log *slog.Logger,
	cameraSaver CameraSaver,
	cameraProvider CameraProvider,
) *CameraHandler {
	return &CameraHandler{
		log:            log,
		cameraSaver:    cameraSaver,
		cameraProvider: cameraProvider,
	}
}

type RequestSave struct {
	CameraIP string `json:"camera_ip" validate:"required"`
	Location string `json:"location" validate:"required"`
	HasAudio *bool  `json:"has_audio" validate:"required"`
}

func (h *CameraHandler) SaveCamera(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.SaveCamera"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req RequestSave
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

	cam, err := h.cameraSaver.SaveCamera(req.CameraIP, req.Location, *req.HasAudio)
	if err != nil {
		if errors.Is(err, errs.ErrCameraAlreadyExists) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("camera already exists", ""))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to save new camera", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, cam)
}

func (h *CameraHandler) Cameras(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.Cameras"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	log.Info("get cameras")

	cams, err := h.cameraProvider.Cameras()
	if err != nil {
		log.Error("failed to get cameras", sl.Err(err))

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to get cameras", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, cams)
}

type RequestUpdate struct {
	Location string `json:"location" validate:"required"`
	HasAudio *bool  `json:"has_audio" validate:"required"`
}

func (h *CameraHandler) UpdateCamera(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.UpdateCamera"

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

	var req RequestUpdate
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

	cam, err := h.cameraProvider.UpdateCamera(cameraID, req.Location, *req.HasAudio)
	if err != nil {
		if errors.Is(err, errs.ErrCameraNotFound) {
			log.Error("camera not found", sl.Err(err))

			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("camera not found", ""))

			return
		}

		log.Error("failed to update camera", sl.Err(err))

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to update camera", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, cam)
}

func (h *CameraHandler) DeleteCamera(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.cameras.DeleteCamera"

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

	log.Info("delete camera", slog.String("camera_id", cameraID))

	err := h.cameraProvider.DeleteCamera(cameraID)
	if err != nil {
		if errors.Is(err, errs.ErrCameraNotFound) {
			log.Error("camera not found", sl.Err(err))

			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, response.Error("camera not found", ""))

			return
		}

		log.Error("failed to delete camera", sl.Err(err))

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to delete camera", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}
