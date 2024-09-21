package recordingstorage

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/storage/postgres"
)

type RecordingStorage struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *RecordingStorage {
	return &RecordingStorage{
		db: db,
	}
}

func (s *RecordingStorage) Create(userID int, cameraIP string) (string, error) {
	const op = "storage.postgres.recordings.Create"

	var id string
	query := fmt.Sprintf(`INSERT INTO %s (user_id, camera_ip, is_moved) VALUES ($1, $2, $3) RETURNING record_id`, postgres.RecordsTable)

	row := s.db.QueryRow(query, userID, cameraIP, false)
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *RecordingStorage) Start(startTime time.Time, filePath, recordID string) error {
	const op = "storage.postgres.recordings.Start"

	query := fmt.Sprintf(`UPDATE %s SET start_time = $1, file_path = $2 WHERE record_id = $3`, postgres.RecordsTable)

	_, err := s.db.Exec(query, startTime, filePath, recordID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordingStorage) Stop(recordID string, stopTime time.Time) error {
	const op = "storage.postgres.recordings.Stop"

	query := fmt.Sprintf(`UPDATE %s SET stop_time = $1 WHERE record_id = $2`, postgres.RecordsTable)

	_, err := s.db.Exec(query, stopTime, recordID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordingStorage) Move(recordID string) error {
	const op = "storage.postgres.recordings.Move"

	query := fmt.Sprintf(`UPDATE %s SET is_moved = true WHERE record_id = $1`, postgres.RecordsTable)

	_, err := s.db.Exec(query, recordID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordingStorage) Recording(recordID string) (models.Recording, error) {
	const op = "storage.postgres.recordings.Recording"

	var rec models.Recording
	query := fmt.Sprintf(`SELECT record_id, camera_ip, start_time, stop_time, file_path, is_moved FROM %s WHERE record_id = $1`, postgres.RecordsTable)

	row := s.db.QueryRow(query, recordID)
	if err := row.Scan(&rec.RecordingID, &rec.CameraIP, &rec.StartTime, &rec.StopTime, &rec.FilePath, &rec.IsMoved); err != nil {
		return models.Recording{}, fmt.Errorf("%s: %w", op, err)
	}

	return rec, nil
}

func (s *RecordingStorage) CameraRecordings(cameraIP string, limit, userID int) ([]models.Recording, error) {
	const op = "storage.postgres.recordings.CameraRecording"

	var recs []models.Recording
	query := fmt.Sprintf(`SELECT record_id, camera_ip, start_time, stop_time, is_moved 
		FROM %s 
		WHERE camera_ip = $1 AND user_id = $2 AND is_moved = false 
		LIMIT $3`, postgres.RecordsTable,
	)

	rows, err := s.db.Query(query, cameraIP, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	for rows.Next() {
		var rec models.Recording
		if err := rows.Scan(&rec.RecordingID, &rec.CameraIP, &rec.StartTime, &rec.StopTime, &rec.IsMoved); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		recs = append(recs, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return recs, nil
}
