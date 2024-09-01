package response

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Response struct {
	Error     string `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func Error(msg, requestID string) Response {
	return Response{
		Error:     msg,
		RequestID: requestID,
	}
}

func ValidationError(errs validator.ValidationErrors) Response {
	var errMsgs []string

	for _, err := range errs {
		switch err.ActualTag() {
		case "required":
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is a required field", err.Field()))
		case "email":
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is not a valid email address", err.Field()))
		case "password":
			errMsgs = append(errMsgs, fmt.Sprintf("field %s does not meet password requirements", err.Field()))
		case "user_type":
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is not a valid user type", err.Field()))
		case "id":
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is not a valid ID", err.Field()))
		default:
			errMsgs = append(errMsgs, fmt.Sprintf("field %s is not valid", err.Field()))
		}
	}

	return Response{
		Error: strings.Join(errMsgs, ", "),
	}
}
