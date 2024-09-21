package models

import "time"

type Recording struct {
	RecordingID string    `json:"recording_id" db:"record_id"`
	CameraIP    string    `json:"camera_ip" db:"camera_ip"`
	StartTime   time.Time `json:"start_time" db:"start_time"`
	StopTime    time.Time `json:"stop_time" db:"stop_time"`
	FilePath    string    `db:"file_path"`
	IsMoved     bool      `json:"is_moved" db:"is_moved"`
}

type ScheduleRecording struct {
	CameraIPs []string  `json:"camera_ips"`
	StartTime time.Time `json:"start_time" validate:"required"`
	Duration  string    `json:"duration" validate:"required"`
}
