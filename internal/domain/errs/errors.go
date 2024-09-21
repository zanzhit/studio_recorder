package errs

import "errors"

var (
	ErrUserType           = errors.New("wrong user type")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrCameraAlreadyExists  = errors.New("camera already exists")
	ErrCameraIsNotAvailable = errors.New("camera is not available")

	ErrRecordNotFound   = errors.New("record not found")
	ErrInvalidStartTime = errors.New("invalid start time")

	ErrWriteToDB = errors.New("failed to write to database")
)
