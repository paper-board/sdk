package httpmw

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"

	pberrs "github.com/paper-board/sdk/errors"
	pblog "github.com/paper-board/sdk/log"
)

// HandleErr writes the canonical error envelope and logs the wrapped chain at
// the HTTP boundary. Code format: "<svc>.<short>" where short comes from
// errors.Kind (not_found, conflict, ..., internal).
//
// Message is the bare sentinel string (e.g. "not found"). The wrapped detail
// is NOT echoed to clients — full chain goes only to logs via log.ErrorAttrs.
//
// If err wraps validator.ValidationErrors, individual field failures appear
// under details with HTTP 400 + invalid_input code.
func HandleErr(w http.ResponseWriter, r *http.Request, svc string, err error) {
	if err == nil {
		return
	}

	var vErrs validator.ValidationErrors
	if errors.As(err, &vErrs) {
		validErr := pberrs.Wrap(pberrs.ErrInvalidInput, "validation failed")
		details := formatValidation(vErrs)
		body := envelope(svc, validErr, details)
		write(w, http.StatusBadRequest, body)
		pblog.Pkg(r.Context()).Error("http error",
			append(pblog.ErrorAttrs(err), "details", details)...,
		)
		return
	}

	status := pberrs.ToHTTPStatus(err)
	body := envelope(svc, err, nil)
	write(w, status, body)
	pblog.Pkg(r.Context()).Error("http error", pblog.ErrorAttrs(err)...)
}

// WriteError emits the canonical envelope without log. For low-level paths
// where logging happened earlier or is intentionally skipped (e.g. healthz).
func WriteError(w http.ResponseWriter, status int, code, message string) {
	write(w, status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

func envelope(svc string, err error, details any) ErrorEnvelope {
	return ErrorEnvelope{Error: ErrorBody{
		Code:    svc + "." + pberrs.Kind(err),
		Message: pberrs.Message(err),
		Details: details,
	}}
}

func write(w http.ResponseWriter, status int, body ErrorEnvelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func formatValidation(vErrs validator.ValidationErrors) []ValidationDetail {
	out := make([]ValidationDetail, 0, len(vErrs))
	for _, fe := range vErrs {
		out = append(out, ValidationDetail{
			Field:   fe.Field(),
			Rule:    fe.Tag(),
			Message: fe.Error(),
		})
	}
	return out
}
