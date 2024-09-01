package recordstorage

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/zanzhit/studio_recorder/internal/storage/postgres"
)

type RecordStorage struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *RecordStorage {
	return &RecordStorage{
		db: db,
	}
}

func (s *RecordStorage) Create(userID int, cameraIP string) (string, error) {
	const op = "storage.postgres.records.Create"

	var id string
	query := fmt.Sprintf(`INSERT INTO %s (user_id, camera_ip, is_moved) VALUES ($1, $2, $3) RETURNING record_id`, postgres.RecordsTable)

	row := s.db.QueryRow(query, userID, cameraIP, false)
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *RecordStorage) Start(startTime time.Time, filePath, recordID string) error {
	const op = "storage.postgres.records.Start"

	var id string
	query := fmt.Sprintf(`UPDATE %s SET start_time = $1, file_path = $2 WHERE record_id = $3`, postgres.RecordsTable)

	row := s.db.QueryRow(query, startTime, filePath, recordID, false)
	if err := row.Scan(&id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordStorage) Stop(recordID string, stopTime time.Time) error {
	const op = "storage.postgres.records.Stop"

	var id string
	query := fmt.Sprintf(`UPDATE %s SET stop_time = &1 WHERE record_id = $2`, postgres.RecordsTable)

	row := s.db.QueryRow(query, stopTime, recordID)
	if err := row.Scan(&id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *RecordStorage) Move(recordID string) error {
	const op = "storage.postgres.records.Move"

	query := fmt.Sprintf(`UPDATE %s SET is_moved = true WHERE record_id = $1`, postgres.RecordsTable)

	_, err := s.db.Exec(query, recordID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
