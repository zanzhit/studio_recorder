package records

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type RecordService struct {
	log          *slog.Logger
	recordSaver  RecordSaver
	videoStorage VideoStorage
	commands     map[string]*exec.Cmd
	videosPath   string
}

type RecordSaver interface {
	Create(userID int, cameraIP string) (string, error)
	Start(startTime time.Time, filePath, recordID string) error
	Stop(recordID string, stopTime time.Time) error
}

type VideoStorage interface {
	Move(models.Recording) error
}

func New(log *slog.Logger, recordSaver RecordSaver, videoStorage VideoStorage, videosPath string) *RecordService {
	return &RecordService{
		log:          log,
		recordSaver:  recordSaver,
		videoStorage: videoStorage,
		commands:     make(map[string]*exec.Cmd),
		videosPath:   videosPath,
	}
}

func (s *RecordService) Start(cameraIPs []string, userID int) (string, error) {
	const op = "service.camera.Start"

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

	recordID, err := s.recordSaver.Create(userID, cameraIPs[0])
	if err != nil {
		log.Error("failed to create record", sl.Err(err))

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

	if err := s.recordSaver.Start(time.Now(), filePath, recordID); err != nil {
		log.Error("failed to write start data", sl.Err(err))

		return recordID, nil
	}

	return recordID, nil
}

func (s *RecordService) Stop(recordID string) error {
	const op = "service.camera.Start"

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

	if err := s.recordSaver.Stop(recordID, time.Now()); err != nil {
		log.Error("failed to write stop data", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordService) Schedule(rec models.ScheduleRecording, userID int) error {
	const op = "service.camera.Schedule"

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

func isCameraAvailable(cameraIP string) (bool, error) {
	conn, err := net.DialTimeout("tcp", cameraIP, 3*time.Second)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}
