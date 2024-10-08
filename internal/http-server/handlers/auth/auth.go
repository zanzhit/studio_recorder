package authhandler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"

	"github.com/zanzhit/studio_recorder/internal/domain/constants"
	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/lib/api/response"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type AuthHandler struct {
	log  *slog.Logger
	user User
}

type User interface {
	Login(email, password string) (string, error)
	RegisterNewUser(email, password, userType string) (string, error)
	UpdatePassword(email, password string) error
	DeleteUser(email string) error
}

func New(
	log *slog.Logger,
	user User,
) *AuthHandler {
	return &AuthHandler{
		log:  log,
		user: user,
	}
}

type Request struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
	UserType string `json:"user_type"`
}

func (h *AuthHandler) RegisterNewUser(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.auth.Register"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req Request
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

	if req.UserType == "" {
		req.UserType = constants.User
	}

	id, err := h.user.RegisterNewUser(req.Email, req.Password, req.UserType)
	if err != nil {
		if errors.Is(err, errs.ErrUserExists) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("user with this email already exists", ""))

			return
		}
		if errors.Is(err, errs.ErrUserType) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid user_type", ""))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to register new user", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, map[string]string{"id": id})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.auth.Login"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req Request
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

	token, err := h.user.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, errs.ErrInvalidCredentials) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("invalid credentials", ""))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to login", middleware.GetReqID(r.Context())))

		return
	}

	render.JSON(w, r, map[string]string{"token": token})
}

type RequestUpdate struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

func (h *AuthHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.auth.UpdatePassword"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

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

	err = h.user.UpdatePassword(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, errs.ErrUserNotFound) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, response.Error("user not found", ""))

			return
		}

		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to update password", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}

type RequestDelete struct {
	Email string `json:"email" validate:"required"`
}

func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	const op = "handlers.auth.DeleteUser"

	log := h.log.With(
		slog.String("op", op),
		slog.String("request_id", middleware.GetReqID(r.Context())),
	)

	var req RequestDelete
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

	err = h.user.DeleteUser(req.Email)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, response.Error("failed to delete user", middleware.GetReqID(r.Context())))

		return
	}

	w.WriteHeader(http.StatusOK)
}
