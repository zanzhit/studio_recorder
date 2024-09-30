package models

type Camera struct {
	CameraID string `json:"camera_id" db:"camera_id"`
	CameraIP string `json:"camera_ip" db:"camera_ip"`
	Location string `json:"location" db:"location"`
	HasAudio bool   `json:"has_audio" db:"has_audio"`
}
