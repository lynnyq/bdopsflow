package datasource

import (
	"errors"
	"testing"
)

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		err  error
		code int
	}{
		{ErrDatasourceNotFound, 3001},
		{ErrDatasourceNameExists, 3002},
		{ErrDatasourceTypeNotSupport, 3003},
		{ErrDatasourceConnFailed, 3004},
		{ErrDatasourceConnTimeout, 3005},
		{ErrDatasourceDisabled, 3006},
		{ErrSQLTypeNotAllowed, 3007},
		{ErrSQLTooLong, 3008},
		{ErrQueryTimeout, 3009},
		{ErrConcurrentLimit, 3010},
		{ErrExportRowLimit, 3011},
		{ErrNoQueryPermission, 3012},
		{ErrNoDownloadPermission, 3013},
		{ErrPermissionExists, 3014},
		{ErrPermissionNotFound, 3026},
		{ErrSavedSQLNotFound, 3015},
		{ErrQueryHistoryNotFound, 3016},
		{ErrCryptoFailed, 3017},
		{ErrConfigInvalid, 3018},
		{ErrQueryCancelled, 3019},
		{ErrQueryExecuteFailed, 3020},
		{ErrPoolExhausted, 3021},
		{ErrServiceUnavailable, 3022},
		{ErrSQLSyntaxError, 3023},
		{ErrMetadataFailed, 3024},
		{ErrPermissionServiceError, 3025},
		{ErrInvalidPermissionType, 3027},
		{ErrDatasourceConcurrentLimit, 3028},
	}

	for _, tt := range tests {
		code := GetErrorCode(tt.err)
		if code != tt.code {
			t.Errorf("GetErrorCode(%v) = %d, want %d", tt.err, code, tt.code)
		}
	}
}

func TestGetErrorCode_UnknownError(t *testing.T) {
	unknownErr := errors.New("some unknown error")
	code := GetErrorCode(unknownErr)
	if code != 3020 {
		t.Errorf("GetErrorCode(unknown) = %d, want 3020", code)
	}
}

func TestGetErrorCode_DatasourceError(t *testing.T) {
	innerErr := errors.New("inner error")
	de := NewDatasourceError(3050, "custom error", innerErr)
	code := GetErrorCode(de)
	if code != 3050 {
		t.Errorf("GetErrorCode(DatasourceError) = %d, want 3050", code)
	}
}

func TestNewDatasourceError(t *testing.T) {
	innerErr := errors.New("inner error")
	de := NewDatasourceError(3001, "datasource not found", innerErr)

	if de.Code != 3001 {
		t.Errorf("expected code 3001, got %d", de.Code)
	}
	if de.Message != "datasource not found" {
		t.Errorf("expected message 'datasource not found', got %q", de.Message)
	}
	if de.Err != innerErr {
		t.Errorf("expected inner error to match")
	}

	errMsg := de.Error()
	if errMsg != "datasource not found" {
		t.Errorf("expected Error() to return 'datasource not found', got %q", errMsg)
	}
}

func TestDatasourceError_Unwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	de := NewDatasourceError(3001, "test", innerErr)

	unwrapped := de.Unwrap()
	if unwrapped != innerErr {
		t.Errorf("expected unwrapped error to match inner error")
	}

	if !errors.Is(de, innerErr) {
		t.Error("expected errors.Is to match inner error via Unwrap")
	}
}

func TestAllErrorsDefined(t *testing.T) {
	errorVars := []error{
		ErrDatasourceNotFound,
		ErrDatasourceNameExists,
		ErrDatasourceTypeNotSupport,
		ErrDatasourceConnFailed,
		ErrDatasourceConnTimeout,
		ErrDatasourceDisabled,
		ErrSQLTypeNotAllowed,
		ErrSQLTooLong,
		ErrQueryTimeout,
		ErrConcurrentLimit,
		ErrExportRowLimit,
		ErrNoQueryPermission,
		ErrNoDownloadPermission,
		ErrPermissionExists,
		ErrPermissionNotFound,
		ErrSavedSQLNotFound,
		ErrQueryHistoryNotFound,
		ErrCryptoFailed,
		ErrConfigInvalid,
		ErrQueryCancelled,
		ErrQueryExecuteFailed,
		ErrPoolExhausted,
		ErrServiceUnavailable,
		ErrSQLSyntaxError,
		ErrMetadataFailed,
		ErrPermissionServiceError,
		ErrInvalidPermissionType,
		ErrDatasourceConcurrentLimit,
		ErrDatasourceCircuitOpen,
	}

	for _, errVar := range errorVars {
		if errVar == nil {
			t.Error("expected error variable to be defined, got nil")
		}
		if errVar.Error() == "" {
			t.Error("expected error variable to have non-empty message")
		}
	}

	if len(errorVars) != len(errorCodes) {
		t.Errorf("number of error variables (%d) does not match errorCodes map entries (%d)", len(errorVars), len(errorCodes))
	}
}
