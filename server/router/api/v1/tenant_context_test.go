package v1

import (
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/usememos/memos/store"
)

func TestGetTenantFromContext(t *testing.T) {
	tests := []struct {
		name     string
		tenantID interface{}
		expected *int32
	}{
		{
			name:     "no tenant set",
			tenantID: nil,
			expected: nil,
		},
		{
			name:     "tenant set",
			tenantID: int32(42),
			expected: func() *int32 { v := int32(42); return &v }(),
		},
		{
			name:     "wrong type",
			tenantID: "not an int32",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := echo.New().NewContext(nil, nil)
			if tt.tenantID != nil {
				c.Set(getTenantIDContextKey(), tt.tenantID)
			}

			result := getTenantFromContext(c)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestSetTenantInContext(t *testing.T) {
	c := echo.New().NewContext(nil, nil)
	tenantID := int32(42)

	setTenantInContext(c, tenantID)

	result := getTenantFromContext(c)
	assert.NotNil(t, result)
	assert.Equal(t, tenantID, *result)
}

func TestApplyTenantFilter(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       interface{}
		expectedFilter *int32
	}{
		{
			name:           "no tenant in context",
			tenantID:       nil,
			expectedFilter: nil,
		},
		{
			name:           "tenant in context",
			tenantID:       int32(42),
			expectedFilter: func() *int32 { v := int32(42); return &v }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := echo.New().NewContext(nil, nil)
			if tt.tenantID != nil {
				c.Set(getTenantIDContextKey(), tt.tenantID)
			}

			find := &store.FindMemo{}
			ApplyTenantFilter(c, find)

			if tt.expectedFilter == nil {
				assert.Nil(t, find.TenantID)
			} else {
				assert.NotNil(t, find.TenantID)
				assert.Equal(t, *tt.expectedFilter, *find.TenantID)
			}
		})
	}
}

func TestApplyTicketTenantFilter(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       interface{}
		expectedFilter *int32
	}{
		{
			name:           "no tenant in context",
			tenantID:       nil,
			expectedFilter: nil,
		},
		{
			name:           "tenant in context",
			tenantID:       int32(42),
			expectedFilter: func() *int32 { v := int32(42); return &v }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := echo.New().NewContext(nil, nil)
			if tt.tenantID != nil {
				c.Set(getTenantIDContextKey(), tt.tenantID)
			}

			find := &store.FindTicket{}
			ApplyTicketTenantFilter(c, find)

			if tt.expectedFilter == nil {
				assert.Nil(t, find.TenantID)
			} else {
				assert.NotNil(t, find.TenantID)
				assert.Equal(t, *tt.expectedFilter, *find.TenantID)
			}
		})
	}
}
