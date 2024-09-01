package models

import "time"

type Recording struct {
	RecordingID string    `json:"recording_id"`
	CameraIP    string    `json:"camera_ip"`
	StartTime   time.Time `json:"start_time"`
	StopTime    time.Time `json:"stop_time"`
	FilePath    string    `json:"file_path"`
	IsMoved     bool      `json:"is_moved"`
}

type ScheduleRecording struct {
	CameraIPs []string  `json:"camera_ips"`
	StartTime time.Time `json:"start_time" validate:"required"`
	Duration  string    `json:"duration" validate:"required"`
}
