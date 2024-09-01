package opencast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
)

type Opencast struct {
	acl        []byte
	processing []byte
	address    string
	login      string
	password   string
}

type Metadata struct {
	Flavor string  `json:"flavor"`
	Fields []Field `json:"fields"`
}

type Field struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
}

const fileExtension = 3

func MustLoad(configPath string) *Opencast {
	if configPath == "" {
		panic("CONFIG_PATH is required")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var opencast Opencast

	if err := cleanenv.ReadConfig(configPath, &opencast); err != nil {
		panic("failed to read config: " + err.Error())
	}

	return &opencast
}

func (o *Opencast) Move(rec models.Recording) error {
	const op = "opencast.Move"

	videoFile, err := os.ReadFile(rec.FilePath)
	if err != nil {
		return fmt.Errorf("%s: failed to read video file: %w", op, err)
	}

	duration := rec.StopTime.Sub(rec.StartTime)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	formattedDuration := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)

	md := []Metadata{
		{
			Flavor: "dublincore/episode",
			Fields: []Field{
				{
					ID:    "title",
					Value: rec.CameraIP,
				},
				{
					ID:    "startDate",
					Value: rec.StartTime.Format(time.DateOnly),
				},
				{
					ID:    "startTime",
					Value: rec.StartTime.Format(time.TimeOnly),
				},
				{
					ID:    "duration",
					Value: formattedDuration,
				},
				{
					ID:    "location",
					Value: rec.CameraIP,
				},
			},
		},
	}

	metadata, err := json.Marshal(md)
	if err != nil {
		return fmt.Errorf("%s: failed to marshal metadata: %w", op, err)
	}

	data := map[string][]byte{
		"presenter":  videoFile,
		"metadata":   metadata,
		"acl":        o.acl,
		"processing": o.processing,
	}

	body := &bytes.Buffer{}
	contentType, err := createForm(data, body, rec)
	if err != nil {
		return fmt.Errorf("%s: failed to create form: %w", op, err)
	}

	opencastVideos := fmt.Sprintf("%s/api/events", o.address)
	req, err := http.NewRequest("POST", opencastVideos, body)
	if err != nil {
		return fmt.Errorf("%s: failed to create request: %w", op, err)
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(o.login, o.password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: failed to send request: %w", op, err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: failed to move video: %s", op, resp.Status)
	}

	return nil
}

func createForm(data map[string][]byte, body *bytes.Buffer, rec models.Recording) (string, error) {
	writer := multipart.NewWriter(body)
	defer writer.Close()

	for fieldName, fieldData := range data {
		if fieldName == "presenter" {
			part, err := writer.CreateFormFile(fieldName, fmt.Sprintf("%s.%s", fieldName, rec.FilePath[len(rec.FilePath)-fileExtension:]))
			if err != nil {
				return "", err
			}

			_, err = io.Copy(part, bytes.NewReader(fieldData))
			if err != nil {
				return "", err
			}

			continue
		}

		part, err := writer.CreateFormField(fieldName)
		if err != nil {
			return "", err
		}
		part.Write(fieldData)
	}

	return writer.FormDataContentType(), nil
}
