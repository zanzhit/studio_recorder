package models

import "time"

type Recording struct {
	RecordingID string    `json:"recording_id" db:"record_id"`
	CameraIP    string    `json:"camera_ip" db:"camera_ip"`
	FilePath    string    `json:"-" db:"file_path"`
	UserID      int       `json:"user_id" db:"user_id"`
	StartTime   time.Time `json:"start_time" db:"start_time"`
	StopTime    time.Time `json:"stop_time" db:"stop_time"`
	IsMoved     bool      `json:"is_moved" db:"is_moved"`
}
