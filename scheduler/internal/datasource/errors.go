package datasource

import "errors"

var (
	ErrDatasourceNotFound       = errors.New("datasource not found")
	ErrDatasourceNameExists     = errors.New("datasource name already exists in this domain")
	ErrDatasourceTypeNotSupport = errors.New("datasource type not supported")
	ErrDatasourceConnFailed     = errors.New("datasource connection failed")
	ErrDatasourceConnTimeout    = errors.New("datasource connection timeout")
	ErrDatasourceDisabled       = errors.New("datasource is disabled")
	ErrSQLTypeNotAllowed        = errors.New("SQL type not allowed, only SELECT is permitted")
	ErrSQLTooLong               = errors.New("SQL text exceeds maximum length")
	ErrQueryTimeout             = errors.New("query execution timeout")
	ErrConcurrentLimit          = errors.New("concurrent query limit exceeded")
	ErrDatasourceConcurrentLimit = errors.New("datasource concurrent query limit exceeded")
	ErrExportRowLimit           = errors.New("export row count exceeds maximum limit")
	ErrNoQueryPermission        = errors.New("no datasource query permission")
	ErrNoDownloadPermission     = errors.New("no datasource download permission")
	ErrPermissionExists         = errors.New("datasource permission already exists")
	ErrPermissionNotFound       = errors.New("datasource permission not found")
	ErrSavedSQLNotFound         = errors.New("saved SQL not found")
	ErrQueryHistoryNotFound     = errors.New("query history not found")
	ErrCryptoFailed             = errors.New("password encryption/decryption failed")
	ErrConfigInvalid            = errors.New("datasource config invalid")
	ErrQueryCancelled           = errors.New("query has been cancelled")
	ErrQueryExecuteFailed       = errors.New("query execution failed")
	ErrPoolExhausted            = errors.New("connection pool exhausted")
	ErrServiceUnavailable       = errors.New("query service temporarily unavailable")
	ErrSQLSyntaxError           = errors.New("SQL syntax error")
	ErrMetadataFailed           = errors.New("metadata fetch failed")
	ErrPermissionServiceError   = errors.New("permission verification service error")
	ErrInvalidPermissionType    = errors.New("invalid permission type")
)

type DatasourceError struct {
	Code    int
	Message string
	Err     error
}

func (e *DatasourceError) Error() string {
	return e.Message
}

func (e *DatasourceError) Unwrap() error {
	return e.Err
}

func NewDatasourceError(code int, message string, err error) *DatasourceError {
	return &DatasourceError{Code: code, Message: message, Err: err}
}

var errorCodes = map[error]int{
	ErrDatasourceNotFound:       3001,
	ErrDatasourceNameExists:     3002,
	ErrDatasourceTypeNotSupport: 3003,
	ErrDatasourceConnFailed:     3004,
	ErrDatasourceConnTimeout:    3005,
	ErrDatasourceDisabled:       3006,
	ErrSQLTypeNotAllowed:        3007,
	ErrSQLTooLong:               3008,
	ErrQueryTimeout:             3009,
	ErrConcurrentLimit:          3010,
	ErrDatasourceConcurrentLimit: 3028,
	ErrExportRowLimit:           3011,
	ErrNoQueryPermission:        3012,
	ErrNoDownloadPermission:     3013,
	ErrPermissionExists:         3014,
	ErrPermissionNotFound:       3026,
	ErrSavedSQLNotFound:         3015,
	ErrQueryHistoryNotFound:     3016,
	ErrCryptoFailed:             3017,
	ErrConfigInvalid:            3018,
	ErrQueryCancelled:           3019,
	ErrQueryExecuteFailed:       3020,
	ErrPoolExhausted:            3021,
	ErrServiceUnavailable:       3022,
	ErrSQLSyntaxError:           3023,
	ErrMetadataFailed:           3024,
	ErrPermissionServiceError:   3025,
	ErrInvalidPermissionType:    3027,
}

func GetErrorCode(err error) int {
	if de, ok := err.(*DatasourceError); ok {
		return de.Code
	}
	if code, ok := errorCodes[err]; ok {
		return code
	}
	return 3020
}
