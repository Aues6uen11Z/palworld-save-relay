// Package apperr provides structured error codes for user-facing operations.
// The frontend extracts the code from the error string (format: "[CODE] detail")
// and maps it to a localized message. Unknown codes fall back to the raw detail.
package apperr

import "fmt"

// Code is a machine-readable error code that the frontend can map to an i18n key.
type Code string

const (
	GameRunning     Code = "GAME_RUNNING"
	QiniuConfig     Code = "QINIU_CONFIG"
	NoCloudVersions Code = "NO_CLOUD_VERSIONS"
	ValidationFail  Code = "VALIDATION_FAILED"
	SteamIDParse    Code = "STEAMID_PARSE"
	BackupFailed    Code = "BACKUP_FAILED"
	PackFailed      Code = "PACK_FAILED"
	UploadFailed    Code = "UPLOAD_FAILED"
	DownloadFailed  Code = "DOWNLOAD_FAILED"
	ConvertFailed   Code = "CONVERT_FAILED"
	StripFailed     Code = "STRIP_FAILED"
	StripFatal      Code = "STRIP_FATAL"
	ReplaceFailed   Code = "REPLACE_FAILED"
	ReplaceFatal    Code = "REPLACE_FATAL"
	RestoreFailed   Code = "RESTORE_FAILED"
	RestoreFatal    Code = "RESTORE_FATAL"
	FileWrite       Code = "FILE_WRITE"
	FileRead        Code = "FILE_READ"
)

// CodedError wraps an underlying error with a Code. Error() returns "[CODE] detail"
// so the frontend can pattern-match the code and show a localized message.
type CodedError struct {
	Code   Code
	Detail string
	Err    error
}

func (e *CodedError) Error() string {
	d := e.Detail
	if e.Err != nil {
		if d != "" {
			d += ": "
		}
		d += e.Err.Error()
	}
	return fmt.Sprintf("[%s] %s", e.Code, d)
}

func (e *CodedError) Unwrap() error { return e.Err }

// New creates a CodedError with a plain detail message (no wrapped error).
func New(code Code, detail string) *CodedError {
	return &CodedError{Code: code, Detail: detail}
}

// Wrap creates a CodedError wrapping err with a code and optional context.
func Wrap(code Code, err error, detail ...string) *CodedError {
	d := ""
	if len(detail) > 0 {
		d = detail[0]
	}
	return &CodedError{Code: code, Detail: d, Err: err}
}
