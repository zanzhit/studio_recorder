package errs

import "errors"

var (
	ErrUserType           = errors.New("wrong user type")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrCameraNotFound       = errors.New("camera not found")
	ErrCameraAlreadyExists  = errors.New("camera already exists")
	ErrCameraIsNotAvailable = errors.New("camera is not available")

	ErrRecordNotFound   = errors.New("record not found")
	ErrFileNotFound     = errors.New("file not found")
	ErrInvalidStartTime = errors.New("invalid start time")
	ErrFileAlreadyMoved = errors.New("file already moved")

	ErrWriteToDB = errors.New("failed to write to database")
)
