package recordingservice

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
	"github.com/google/uuid"
	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"
)

type RecordingService struct {
	log               *slog.Logger
	recordingSaver    RecordingSaver
	recordingProvider RecordingProvider
	cameraProvider    CameraProvider
	videoService      VideoService
	commands          map[string]*exec.Cmd
	videosPath        string
}

type CameraProvider interface {
	CameraIP(cameraID string) (string, error)
}

type RecordingSaver interface {
	Start(recording models.Recording, cameraID string) error
	Stop(recordID string, stopTime time.Time) error
}

type RecordingProvider interface {
	CameraRecordings(cameraID string, limit, offset, userID int) ([]models.Recording, error)
	Recording(recordID string) (models.Recording, error)
	Move(recordID string) error
	Delete(recordID string) error
}

type VideoService interface {
	Move(models.Recording) error
}

func New(log *slog.Logger, recordingSaver RecordingSaver, recordingProvider RecordingProvider, cameraProvider CameraProvider, videoService VideoService, videosPath string) *RecordingService {
	return &RecordingService{
		log:               log,
		recordingSaver:    recordingSaver,
		recordingProvider: recordingProvider,
		cameraProvider:    cameraProvider,
		videoService:      videoService,
		commands:          make(map[string]*exec.Cmd),
		videosPath:        videosPath,
	}
}

type camera struct {
	cameraIP string
	audio    bool
}

func (s *RecordingService) Start(cameraIDs []string, userID int) (string, error) {
	const op = "service.recordings.Start"

	log := s.log.With(
		slog.String("op", op),
		slog.String("camera_id", strings.Join(cameraIDs, ", ")),
		slog.Int("user_id", userID),
	)

	var cameras []*camera
	for _, cameraID := range cameraIDs {
		cameraIP, err := s.cameraProvider.CameraIP(cameraID)
		if err != nil {
			log.Error("failed to get camera ip", sl.Err(err))

			return "", fmt.Errorf("%s: %w", op, err)
		}

		cameras = append(cameras, &camera{cameraIP: cameraIP})
	}

	for _, cam := range cameras {
		var err error
		cam.audio, err = isCameraAvailable(cam.cameraIP)
		if err != nil {
			log.Error("camera is not available", sl.Err(err))

			return "", fmt.Errorf("%s: %w", op, errs.ErrCameraIsNotAvailable)
		}
	}

	rec := models.Recording{
		RecordingID: uuid.New().String(),
		UserID:      userID,
		StartTime:   time.Now(),
	}

	log.Info("start recording", slog.String("record_id", rec.RecordingID))

	rec.FilePath = fmt.Sprintf("%s/%s/%s_%s.mkv", s.videosPath, cameraIDs[0], rec.RecordingID, rec.StartTime.Format("2006-01-02_15-04-05"))

	parametres, err := recordingMode(cameras, rec.FilePath)
	if err != nil {
		log.Error("failed to select recording mode", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	cmd := exec.Command(parametres[0], parametres[1:]...)
	if err := cmd.Start(); err != nil {
		log.Error("failed to start recording", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	s.commands[rec.RecordingID] = cmd

	if err := s.recordingSaver.Start(rec, cameraIDs[0]); err != nil {
		log.Error("failed to write start data", sl.Err(err))

		return rec.RecordingID, errs.ErrWriteToDB
	}

	return rec.RecordingID, nil
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

func (s *RecordingService) Schedule(startTime time.Time, cameraID []string, duration string, userID int) error {
	const op = "service.recordings.Schedule"

	log := s.log.With(
		slog.String("op", op),
		slog.String("camera_id", strings.Join(cameraID, ", ")),
		slog.Int("user_id", userID),
	)

	log.Info("schedule recording", slog.Any("schedule", cameraID), slog.Any("start_time", startTime), slog.Any("duration", duration))

	delay := time.Until(startTime)
	if delay < 0 {
		log.Error("invalid start time", slog.Any("start_time", startTime))

		return fmt.Errorf("%s: %w", op, errs.ErrInvalidStartTime)
	}

	durationTime, err := time.ParseDuration(duration)
	if err != nil {
		log.Error("wrong duration format", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	var recordID string

	time.AfterFunc(delay, func() {
		if recordID, err = s.Start(cameraID, userID); err != nil {
			log.Error("failed to start recording", sl.Err(err))

			return
		}

		time.AfterFunc(durationTime, func() {
			if err := s.Stop(recordID); err != nil {
				log.Error("failed to stop recording", sl.Err(err))

				return
			}
		})
	})

	return nil
}

func (s *RecordingService) CameraRecordings(cameraID string, limit, offset, userID int) ([]models.Recording, error) {
	const op = "service.recordings.CameraRecordings"

	log := s.log.With(
		slog.String("op", op),
		slog.String("camera_id", cameraID),
		slog.Int("limit", limit),
		slog.Int("user_id", userID),
	)

	log.Info("get camera recordings", slog.String("camera_id", cameraID))

	recs, err := s.recordingProvider.CameraRecordings(cameraID, limit, offset, userID)
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

func (s *RecordingService) Delete(recordID string) error {
	const op = "service.recordings.Delete"

	log := s.log.With(
		slog.String("op", op),
		slog.String("record_id", recordID),
	)

	log.Info("delete recording", slog.String("record_id", recordID))

	rec, err := s.recordingProvider.Recording(recordID)
	if err != nil {
		log.Error("failed to get recording", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	if !rec.IsMoved {
		if err = os.Remove(rec.FilePath); err != nil {
			log.Error("failed to delete file", sl.Err(err))

			return fmt.Errorf("%s: %w", op, err)
		}
	}

	if err = s.recordingProvider.Delete(recordID); err != nil {
		log.Error("failed to delete record", sl.Err(err))

		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordingService) File(recordID string) (string, error) {
	const op = "service.recordings.Download"

	log := s.log.With(
		slog.String("op", op),
		slog.String("record_id", recordID),
	)

	log.Info("get file", slog.String("record_id", recordID))

	rec, err := s.recordingProvider.Recording(recordID)
	if err != nil {
		log.Error("failed to get recording", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	if rec.IsMoved {
		log.Error("file already moved")

		return "", fmt.Errorf("%s: %w", op, errs.ErrFileAlreadyMoved)
	}

	if _, err := os.Stat(rec.FilePath); err != nil {
		log.Error("file not found", sl.Err(err))

		if err = s.recordingProvider.Delete(recordID); err != nil {
			log.Error("failed to delete record", sl.Err(err))

			return "", fmt.Errorf("%s: %w", op, errs.ErrFileNotFound)
		}

		return "", fmt.Errorf("%s: %w", op, errs.ErrFileNotFound)
	}

	return rec.FilePath, nil
}

func recordingMode(cameras []*camera, filePath string) ([]string, error) {
	var parametres string
	switch len(cameras) {
	case 1:
		if cameras[0].audio {
			parametres = fmt.Sprintf("gst-launch-1.0 uridecodebin uri=%s name=dec ! queue ! videoconvert ! x264enc ! matroskamux name=mux ! filesink location=%s dec. ! queue ! audioconvert ! lamemp3enc ! mux.",
				cameras[0].cameraIP, filePath)
		} else {
			parametres = fmt.Sprintf("gst-launch-1.0 rtspsrc location=%s ! rtph264depay ! h264parse ! matroskamux ! filesink location=\"%s\"",
				cameras[0].cameraIP, filePath)
		}
	case 2:
		if cameras[0].audio {
			parametres = fmt.Sprintf("gst-launch-1.0 rtspsrc location=%s name=src ! rtph264depay ! h264parse ! queue ! mux. src. ! rtpmp4gdepay ! aacparse ! queue ! mux. matroskamux name=mux ! filesink location=\"%s\"",
				cameras[0].cameraIP, filePath)

		} else {
			parametres = fmt.Sprintf("gst-launch-1.0 -e videomixer name=mix sink_0::xpos=0 sink_1::xpos=640 ! videoconvert ! x264enc ! queue ! mux. uridecodebin uri=%s ! videoconvert ! videoscale ! video/x-raw,width=640,height=480 ! mix.sink_0 uridecodebin uri=%s ! videoconvert ! videoscale ! video/x-raw,width=640,height=480 ! mix.sink_1 matroskamux name=mux ! filesink location=%s",
				cameras[0].cameraIP, cameras[1].cameraIP, filePath)
		}
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

	conn := gortsplib.Client{ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second}

	err = conn.Start(u.Scheme, u.Host)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	tracks, _, _, err := conn.Describe(u)
	if err != nil {
		return false, err
	}

	audioFound := false

	for _, track := range tracks {
		switch track.(type) {
		case *gortsplib.TrackOpus, *gortsplib.TrackVorbis, *gortsplib.TrackG711, *gortsplib.TrackG722, *gortsplib.TrackMPEG2Audio, *gortsplib.TrackMPEG4Audio:
			audioFound = true

			return audioFound, nil
		}
	}

	return audioFound, nil
}
