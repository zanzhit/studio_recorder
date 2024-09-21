package recordingservice

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type RecordingService struct {
	log               *slog.Logger
	recordingSaver    RecordingSaver
	recordingProvider RecordingProvider
	videoService      VideoService
	commands          map[string]*exec.Cmd
	videosPath        string
}

type RecordingSaver interface {
	Create(userID int, cameraIP string) (string, error)
	Start(startTime time.Time, filePath, recordID string) error
	Stop(recordID string, stopTime time.Time) error
}

type RecordingProvider interface {
	CameraRecordings(cameraIP string, limit, userID int) ([]models.Recording, error)
	Recording(recordID string) (models.Recording, error)
	Move(recordID string) error
}

type VideoService interface {
	Move(models.Recording) error
}

func New(log *slog.Logger, recordingSaver RecordingSaver, recordingProvider RecordingProvider, videoService VideoService, videosPath string) *RecordingService {
	return &RecordingService{
		log:               log,
		recordingSaver:    recordingSaver,
		recordingProvider: recordingProvider,
		videoService:      videoService,
		commands:          make(map[string]*exec.Cmd),
		videosPath:        videosPath,
	}
}

func (s *RecordingService) Start(cameraIPs []string, userID int) (string, error) {
	const op = "service.recordings.Start"

	log := s.log.With(
		slog.String("op", op),
		slog.String("camera_ip", strings.Join(cameraIPs, ", ")),
		slog.Int("user_id", userID),
	)

	for _, cameraIP := range cameraIPs {
		available, err := isCameraAvailable(cameraIP)
		if err != nil {
			log.Error("failed to check camera availability", sl.Err(err))

			return "", fmt.Errorf("%s: %w", op, err)
		}

		if !available {
			log.Error("camera is not available", slog.String("camera_ip", cameraIP))

			return "", fmt.Errorf("%s: %w", op, errs.ErrCameraIsNotAvailable)
		}
	}

	log.Info("create record")

	recordID, err := s.recordingSaver.Create(userID, cameraIPs[0])
	if err != nil {
		log.Error("failed to create recording", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	log.Info("start recording", slog.String("record_id", recordID))

	filePath := fmt.Sprintf("%s/%s_%s.mkv", s.videosPath, recordID, time.Now().Format("2006-01-02_15-04-05"))

	parametres, err := recordingMode(cameraIPs, filePath)
	if err != nil {
		log.Error("failed to select recording mode", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	cmd := exec.Command(parametres[0], parametres[1:]...)
	if err := cmd.Start(); err != nil {
		log.Error("failed to start recording", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	s.commands[recordID] = cmd

	if err := s.recordingSaver.Start(time.Now(), filePath, recordID); err != nil {
		log.Error("failed to write start data", sl.Err(err))

		return recordID, errs.ErrWriteToDB
	}

	return recordID, nil
}

func (s *RecordingService) Stop(recordID string) error {
	const op = "service.recordings.Stop"

	log := s.log.With(
		slog.String("op", op),
		slog.String("record_id", recordID),
	)

	log.Info("stop recording", slog.String("record_id", recordID))

	cmd, ok := s.commands[recordID]
	if !ok {
		log.Error("record not found", slog.String("record_id", recordID))

		return fmt.Errorf("%s: %w", op, errs.ErrRecordNotFound)
	}

	if err := cmd.Process.Kill(); err != nil {
		log.Error("failed to stop recording", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("record successfully stopped")

	if err := s.recordingSaver.Stop(recordID, time.Now()); err != nil {
		log.Error("failed to write stop data", sl.Err(err))

		return errs.ErrWriteToDB
	}

	return nil
}

func (s *RecordingService) Schedule(rec models.ScheduleRecording, userID int) error {
	const op = "service.recordings.Schedule"

	log := s.log.With(
		slog.String("op", op),
		slog.String("camera_ip", strings.Join(rec.CameraIPs, ", ")),
		slog.Int("user_id", userID),
	)

	log.Info("schedule recording", slog.Any("schedule", rec))

	delay := time.Until(rec.StartTime)
	if delay < 0 {
		log.Error("invalid start time", slog.Any("start_time", rec.StartTime))

		return fmt.Errorf("%s: %w", op, errs.ErrInvalidStartTime)
	}

	duration, err := time.ParseDuration(rec.Duration)
	if err != nil {
		log.Error("wrong duration format", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	var recordID string

	time.AfterFunc(delay, func() {
		if recordID, err = s.Start(rec.CameraIPs, userID); err != nil {
			log.Error("failed to start recording", sl.Err(err))

			return
		}

		time.AfterFunc(duration, func() {
			if err := s.Stop(recordID); err != nil {
				log.Error("failed to stop recording", sl.Err(err))

				return
			}
		})
	})

	return nil
}

func (s *RecordingService) CameraRecordings(cameraIP string, limit, userID int) ([]models.Recording, error) {
	const op = "service.recordings.CameraRecordings"

	log := s.log.With(
		slog.String("op", op),
		slog.String("camera_ip", cameraIP),
		slog.Int("limit", limit),
		slog.Int("user_id", userID),
	)

	log.Info("get camera recordings", slog.String("camera_ip", cameraIP))

	recs, err := s.recordingProvider.CameraRecordings(cameraIP, limit, userID)
	if err != nil {
		log.Error("failed to get recordings", sl.Err(err))

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return recs, nil
}

func (s *RecordingService) Move(recordingID string) error {
	const op = "service.recordings.Move"

	log := s.log.With(
		slog.String("op", op),
		slog.String("record_id", recordingID),
	)

	log.Info("move recording", slog.String("record_id", recordingID))

	rec, err := s.recordingProvider.Recording(recordingID)
	if err != nil {
		log.Error("failed to get recording", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.videoService.Move(rec); err != nil {
		log.Error("failed to move recording", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.recordingProvider.Move(recordingID); err != nil {
		log.Error("failed to write move data", sl.Err(err))

		return errs.ErrWriteToDB
	}

	return nil
}

func recordingMode(cameraIPs []string, filePath string) ([]string, error) {
	var parametres string
	switch len(cameraIPs) {
	case 1:
		parametres = fmt.Sprintf("gst-launch-1.0 rtspsrc location=%s ! rtph264depay ! h264parse ! matroskamux ! filesink location=%s",
			cameraIPs[0], filePath)
	case 2:
		parametres = fmt.Sprintf("gst-launch-1.0 -e videomixer name=mix sink_0::xpos=0 sink_1::xpos=640 ! videoconvert ! x264enc ! queue ! mux. uridecodebin uri=%s ! videoconvert ! videoscale ! video/x-raw,width=640,height=480 ! mix.sink_0 uridecodebin uri=%s ! videoconvert ! videoscale ! video/x-raw,width=640,height=480 ! mix.sink_1 uridecodebin uri=%s ! audioconvert ! vorbisenc ! queue ! mux. matroskamux name=mux ! filesink location=%s",
			cameraIPs[0], cameraIPs[1], cameraIPs[0], filePath)
	default:
		return nil, fmt.Errorf("too many arguments in camera_ips")
	}

	return strings.Split(parametres, " "), nil
}

func isCameraAvailable(rtspURL string) (bool, error) {
	u, err := url.Parse(rtspURL)
	if err != nil {
		return false, err
	}

	conn := gortsplib.Client{}

	err = conn.Start(u.Scheme, u.Host)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	_, err = conn.Options(u)
	if err != nil {
		return false, err
	}

	return true, nil
}
