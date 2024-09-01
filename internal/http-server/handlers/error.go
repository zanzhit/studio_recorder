package handlers

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/zanzhit/studio_recorder/internal/lib/api/response"
)

func Error(w http.ResponseWriter, r *http.Request, statusCode int, err response.Response) {
	w.WriteHeader(statusCode)
	render.JSON(w, r, err)
}
