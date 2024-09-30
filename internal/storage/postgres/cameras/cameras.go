package camerastorage

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/storage/postgres"
)

type CameraStorage struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *CameraStorage {
	return &CameraStorage{
		db: db,
	}
}

func (s *CameraStorage) SaveCamera(cam models.Camera) (models.Camera, error) {
	const op = "storage.postgres.cameras.Save"

	query := fmt.Sprintf(`INSERT INTO %s (camera_id, camera_ip, location, has_audio) VALUES ($1, $2, $3, $4) RETURNING *`, postgres.CamerasTable)

	err := s.db.QueryRowx(query, cam.CameraID, cam.CameraIP, cam.Location, cam.HasAudio).StructScan(&cam)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return cam, fmt.Errorf("%s: %w", op, errs.ErrCameraAlreadyExists)
		}

		return cam, fmt.Errorf("%s: %w", op, err)
	}

	return cam, nil
}

func (s *CameraStorage) CameraIP(cameraID string) (string, error) {
	const op = "storage.postgres.cameras.CameraIP"

	query := fmt.Sprintf(`SELECT camera_ip FROM %s WHERE camera_id = $1`, postgres.CamerasTable)

	var cameraIP string
	err := s.db.Get(&cameraIP, query, cameraID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cameraIP, fmt.Errorf("%s: %w", op, errs.ErrCameraNotFound)
		}
		return cameraIP, fmt.Errorf("%s: %w", op, err)
	}

	return cameraIP, nil
}

func (s *CameraStorage) Cameras() ([]models.Camera, error) {
	const op = "storage.postgres.cameras.Cameras"

	query := fmt.Sprintf(`SELECT * FROM %s`, postgres.CamerasTable)

	var cameras []models.Camera
	err := s.db.Select(&cameras, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return cameras, nil
}

func (s *CameraStorage) UpdateCamera(cameraID, location string, hasAudio bool) (models.Camera, error) {
	const op = "storage.postgres.cameras.Update"

	query := fmt.Sprintf(`UPDATE %s SET location = $1, has_audio = $2 WHERE camera_id = $3 RETURNING *`, postgres.CamerasTable)

	var cam models.Camera

	err := s.db.QueryRowx(query, location, hasAudio, cameraID).StructScan(&cam)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cam, fmt.Errorf("%s: %w", op, errs.ErrCameraNotFound)
		}
		return cam, fmt.Errorf("%s: %w", op, err)
	}

	return cam, nil
}

func (s *CameraStorage) DeleteCamera(cameraID string) error {
	const op = "storage.postgres.cameras.Delete"

	query := fmt.Sprintf(`DELETE FROM %s WHERE camera_id = $1`, postgres.CamerasTable)

	result, err := s.db.Exec(query, cameraID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%s: %w", op, errs.ErrCameraNotFound)
	}

	return nil
}
