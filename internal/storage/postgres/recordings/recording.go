package recordingstorage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/zanzhit/studio_recorder/internal/domain/errs"
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

func (s *RecordingStorage) Start(rec models.Recording, cameraID string) error {
	const op = "storage.postgres.recordings.Start"

	query := fmt.Sprintf(`INSERT INTO %s (record_id, user_id, camera_id, start_time, file_path, is_moved) 
		VALUES ($1, $2, $3, $4, $5, $6)`, postgres.RecordsTable)

	_, err := s.db.Exec(query, rec.RecordingID, rec.UserID, cameraID, rec.StartTime, rec.FilePath, false)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordingStorage) Stop(recordID string, stopTime time.Time) error {
	const op = "storage.postgres.recordings.Stop"

	query := fmt.Sprintf(`UPDATE %s SET stop_time = $1 WHERE record_id = $2`, postgres.RecordsTable)

	result, err := s.db.Exec(query, stopTime, recordID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: %w", op, errs.ErrRecordNotFound)
	}

	return nil
}

func (s *RecordingStorage) Move(recordID string) error {
	const op = "storage.postgres.recordings.Move"

	query := fmt.Sprintf(`UPDATE %s SET is_moved = true WHERE record_id = $1`, postgres.RecordsTable)

	result, err := s.db.Exec(query, recordID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: %w", op, errs.ErrRecordNotFound)
	}

	return nil
}

func (s *RecordingStorage) Delete(recordID string) error {
	const op = "storage.postgres.recordings.Delete"

	query := fmt.Sprintf(`DELETE FROM %s WHERE record_id = $1`, postgres.RecordsTable)

	result, err := s.db.Exec(query, recordID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: %w", op, errs.ErrRecordNotFound)
	}

	return nil
}

func (s *RecordingStorage) Recording(recordID string) (models.Recording, error) {
	const op = "storage.postgres.recordings.Recording"

	var rec models.Recording
	var stopTime sql.NullTime

	query := fmt.Sprintf(`
		SELECT r.record_id, c.camera_ip, r.start_time, r.stop_time, r.file_path, r.is_moved
		FROM %s r
		JOIN %s c ON r.camera_id = c.camera_id
		WHERE r.record_id = $1`, postgres.RecordsTable, postgres.CamerasTable)

	row := s.db.QueryRow(query, recordID)
	if err := row.Scan(&rec.RecordingID, &rec.CameraIP, &rec.StartTime, &stopTime, &rec.FilePath, &rec.IsMoved); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Recording{}, fmt.Errorf("%s: %w", op, errs.ErrRecordNotFound)
		}
		return models.Recording{}, fmt.Errorf("%s: %w", op, err)
	}

	if stopTime.Valid {
		rec.StopTime = stopTime.Time

		return rec, nil
	}

	rec.StopTime = time.Time{}

	return rec, nil
}

func (s *RecordingStorage) CameraRecordings(cameraID string, limit, offset, userID int) ([]models.Recording, error) {
	const op = "storage.postgres.recordings.CameraRecording"

	var recs []models.Recording
	query := fmt.Sprintf(`
		SELECT r.record_id, c.camera_ip, r.user_id, r.start_time, r.stop_time, r.is_moved
		FROM %s r
		JOIN %s c ON r.camera_id = c.camera_id
		WHERE r.camera_id = $1 AND r.user_id = $2
		LIMIT $3 OFFSET $4`, postgres.RecordsTable, postgres.CamerasTable)

	rows, err := s.db.Query(query, cameraID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	for rows.Next() {
		var rec models.Recording
		var stopTime sql.NullTime

		if err := rows.Scan(&rec.RecordingID, &rec.CameraIP, &rec.UserID, &rec.StartTime, &stopTime, &rec.IsMoved); err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}

		if stopTime.Valid {
			rec.StopTime = stopTime.Time
		} else {
			rec.StopTime = time.Time{}
		}

		recs = append(recs, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return recs, nil
}
