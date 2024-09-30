package cameraservice

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lithammer/shortuuid/v3"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type CameraService struct {
	log         *slog.Logger
	videosPath  string
	cameraSaver CameraSaver
}

type CameraSaver interface {
	SaveCamera(cam models.Camera) (models.Camera, error)
}

func New(log *slog.Logger, videosPath string, cameraSaver CameraSaver) *CameraService {
	return &CameraService{
		log:         log,
		videosPath:  videosPath,
		cameraSaver: cameraSaver,
	}
}

func (s *CameraService) SaveCamera(cameraIP, location string, hasAudio bool) (models.Camera, error) {
	const op = "service.cameras.SaveCamera"

	log := s.log.With(
		slog.String("op", op),
		slog.String("camera_ip", cameraIP),
	)

	log.Info("save camera", slog.String("camera_ip", cameraIP))

	cam := models.Camera{
		CameraID: shortuuid.New(),
		CameraIP: cameraIP,
		Location: location,
		HasAudio: hasAudio,
	}

	cam, err := s.cameraSaver.SaveCamera(cam)
	if err != nil {
		log.Error("failed to save camera", sl.Err(err))

		return models.Camera{}, err
	}

	dirPath := filepath.Join(s.videosPath, cam.CameraID)
	err = os.MkdirAll(dirPath, os.ModePerm)
	if err != nil {
		log.Error("failed to create directory", sl.Err(err))

		return models.Camera{}, err
	}

	return cam, nil
}
