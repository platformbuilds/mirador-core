package errors

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppError_Error(t *testing.T) {
	t.Run("without cause", func(t *testing.T) {
		err := New(CategoryValidation, "INVALID_INPUT", "input is invalid")
		assert.Equal(t, "INVALID_INPUT: input is invalid", err.Error())
	})

	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := New(CategoryInternal, "INTERNAL", "something failed").WithCause(cause)
		assert.Contains(t, err.Error(), "underlying error")
	})
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	err := InternalWithCause("wrapped", cause)

	assert.True(t, errors.Is(err, cause))

	var appErr *AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, "INTERNAL_ERROR", appErr.Code)
}

func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		category Category
		expected int
	}{
		{CategoryValidation, http.StatusBadRequest},
		{CategoryNotFound, http.StatusNotFound},
		{CategoryConflict, http.StatusConflict},
		{CategoryUnauthorized, http.StatusUnauthorized},
		{CategoryForbidden, http.StatusForbidden},
		{CategoryTimeout, http.StatusGatewayTimeout},
		{CategoryUnavailable, http.StatusServiceUnavailable},
		{CategoryBadGateway, http.StatusBadGateway},
		{CategoryInternal, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			err := New(tt.category, "TEST", "test message")
			assert.Equal(t, tt.expected, err.HTTPStatus())
		})
	}
}

func TestAppError_ToResponse(t *testing.T) {
	err := &AppError{
		Category: CategoryValidation,
		Code:     "INVALID_FIELD",
		Message:  "field is invalid",
		Details:  "email",
	}

	resp := err.ToResponse()
	assert.Equal(t, "field is invalid", resp.Error)
	assert.Equal(t, "INVALID_FIELD", resp.Code)
	assert.Equal(t, "email", resp.Details)
}

func TestValidation(t *testing.T) {
	err := Validation("INVALID_EMAIL", "email format is invalid")
	assert.Equal(t, CategoryValidation, err.Category)
	assert.Equal(t, "INVALID_EMAIL", err.Code)
	assert.Equal(t, http.StatusBadRequest, err.HTTPStatus())
}

func TestInvalidField(t *testing.T) {
	err := InvalidField("email", "must be a valid email address")
	assert.Equal(t, CategoryValidation, err.Category)
	assert.Equal(t, "INVALID_FIELD", err.Code)
	assert.Contains(t, err.Message, "email")
	assert.Contains(t, err.Message, "must be a valid email address")
	assert.Equal(t, "email", err.Details)
}

func TestMissingField(t *testing.T) {
	err := MissingField("name")
	assert.Equal(t, CategoryValidation, err.Category)
	assert.Equal(t, "MISSING_FIELD", err.Code)
	assert.Contains(t, err.Message, "name")
}

func TestNotFound(t *testing.T) {
	err := NotFound("USER", "user-123")
	assert.Equal(t, CategoryNotFound, err.Category)
	assert.Equal(t, "USER_NOT_FOUND", err.Code)
	assert.Contains(t, err.Message, "user-123")
}

func TestKPINotFound(t *testing.T) {
	err := KPINotFound("kpi-456")
	assert.Equal(t, CategoryNotFound, err.Category)
	assert.Equal(t, "KPI_NOT_FOUND", err.Code)
	assert.Contains(t, err.Message, "kpi-456")
}

func TestDataSourceNotFound(t *testing.T) {
	err := DataSourceNotFound("ds-789")
	assert.Equal(t, CategoryNotFound, err.Category)
	assert.Equal(t, "DATA_SOURCE_NOT_FOUND", err.Code)
}

func TestConflict(t *testing.T) {
	err := Conflict("KPI", "KPI with this name already exists")
	assert.Equal(t, CategoryConflict, err.Category)
	assert.Equal(t, "KPI_CONFLICT", err.Code)
}

func TestAlreadyExists(t *testing.T) {
	err := AlreadyExists("USER", "john@example.com")
	assert.Equal(t, CategoryConflict, err.Category)
	assert.Equal(t, "USER_EXISTS", err.Code)
	assert.Contains(t, err.Message, "john@example.com")
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("invalid credentials")
	assert.Equal(t, CategoryUnauthorized, err.Category)
	assert.Equal(t, "UNAUTHORIZED", err.Code)
	assert.Equal(t, http.StatusUnauthorized, err.HTTPStatus())
}

func TestForbidden(t *testing.T) {
	err := Forbidden("insufficient permissions")
	assert.Equal(t, CategoryForbidden, err.Category)
	assert.Equal(t, "FORBIDDEN", err.Code)
	assert.Equal(t, http.StatusForbidden, err.HTTPStatus())
}

func TestInternal(t *testing.T) {
	err := Internal("unexpected error occurred")
	assert.Equal(t, CategoryInternal, err.Category)
	assert.Equal(t, "INTERNAL_ERROR", err.Code)
	assert.Equal(t, http.StatusInternalServerError, err.HTTPStatus())
}

func TestTimeout(t *testing.T) {
	err := Timeout("database query")
	assert.Equal(t, CategoryTimeout, err.Category)
	assert.Equal(t, "TIMEOUT", err.Code)
	assert.Contains(t, err.Message, "database query")
}

func TestUnavailable(t *testing.T) {
	err := Unavailable("cache")
	assert.Equal(t, CategoryUnavailable, err.Category)
	assert.Equal(t, "SERVICE_UNAVAILABLE", err.Code)
}

func TestDatabaseUnavailable(t *testing.T) {
	err := DatabaseUnavailable("MariaDB")
	assert.Equal(t, CategoryUnavailable, err.Category)
	assert.Equal(t, "DATABASE_UNAVAILABLE", err.Code)
	assert.Contains(t, err.Message, "MariaDB")
}

func TestIsNotFound(t *testing.T) {
	t.Run("app error not found", func(t *testing.T) {
		err := NotFound("KPI", "123")
		assert.True(t, IsNotFound(err))
	})

	t.Run("app error other category", func(t *testing.T) {
		err := Validation("INVALID", "invalid")
		assert.False(t, IsNotFound(err))
	})

	t.Run("non-app error", func(t *testing.T) {
		err := errors.New("some error")
		assert.False(t, IsNotFound(err))
	})
}

func TestIsValidation(t *testing.T) {
	t.Run("validation error", func(t *testing.T) {
		err := InvalidField("email", "invalid")
		assert.True(t, IsValidation(err))
	})

	t.Run("not validation error", func(t *testing.T) {
		err := Internal("server error")
		assert.False(t, IsValidation(err))
	})
}

func TestIsUnavailable(t *testing.T) {
	t.Run("unavailable error", func(t *testing.T) {
		err := Unavailable("service")
		assert.True(t, IsUnavailable(err))
	})

	t.Run("not unavailable error", func(t *testing.T) {
		err := NotFound("KPI", "123")
		assert.False(t, IsUnavailable(err))
	})
}

func TestGetHTTPStatus(t *testing.T) {
	t.Run("app error", func(t *testing.T) {
		err := NotFound("KPI", "123")
		assert.Equal(t, http.StatusNotFound, GetHTTPStatus(err))
	})

	t.Run("non-app error", func(t *testing.T) {
		err := errors.New("some error")
		assert.Equal(t, http.StatusInternalServerError, GetHTTPStatus(err))
	})
}

func TestToErrorResponse(t *testing.T) {
	t.Run("app error", func(t *testing.T) {
		err := InvalidField("email", "must be valid")
		resp := ToErrorResponse(err)
		assert.Equal(t, "INVALID_FIELD", resp.Code)
		assert.Contains(t, resp.Error, "email")
	})

	t.Run("non-app error", func(t *testing.T) {
		err := errors.New("generic error")
		resp := ToErrorResponse(err)
		assert.Equal(t, "INTERNAL_ERROR", resp.Code)
		assert.Equal(t, "generic error", resp.Error)
	})
}

func TestWrap(t *testing.T) {
	cause := errors.New("connection refused")
	err := Wrap(cause, "DB_CONNECTION", "failed to connect to database")

	assert.Equal(t, "DB_CONNECTION", err.Code)
	assert.Contains(t, err.Message, "database")
	assert.True(t, errors.Is(err, cause))
}

func TestWithDetails(t *testing.T) {
	err := Validation("INVALID", "invalid input").WithDetails("field: email, expected: string")
	assert.Equal(t, "field: email, expected: string", err.Details)
}
