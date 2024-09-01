package camerastorage

import (
	"fmt"

	"github.com/jmoiron/sqlx"
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

func (s *CameraStorage) Save(cam models.Camera) (models.Camera, error) {
	const op = "storage.postgres.cameras.Save"

	query := fmt.Sprintf(`INSERT INTO %s (camera_ip, location, has_audio) VALUES ($1, $2, $3) RETURNING *`, postgres.CamerasTable)

	err := s.db.QueryRowx(query, cam.CameraIP, cam.Location, cam.HasAudio).StructScan(&cam)
	if err != nil {
		return cam, fmt.Errorf("%s: %w", op, err)
	}

	return cam, nil
}
