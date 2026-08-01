package auth_test

import (
	"strings"
	"testing"

	"github.com/richxcame/ride-hailing/internal/auth"
	"github.com/richxcame/ride-hailing/pkg/common"
	"github.com/richxcame/ride-hailing/pkg/models"
	"github.com/stretchr/testify/assert"
)

// baseRegisterRequest returns a valid RegisterRequest for use in validation tests.
func baseRegisterRequest() *models.RegisterRequest {
	return &models.RegisterRequest{
		Email:       "user@example.com",
		Password:    "SecurePass1",
		PhoneNumber: "+1234567890",
		FirstName:   "John",
		LastName:    "Doe",
		Role:        models.RoleRider,
	}
}

func TestValidateRegisterRequest(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(req *models.RegisterRequest)
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid request passes",
			modify:  func(_ *models.RegisterRequest) {},
			wantErr: false,
		},
		{
			name:        "invalid email fails",
			modify:      func(r *models.RegisterRequest) { r.Email = "not-an-email" },
			wantErr:     true,
			errContains: "invalid email",
		},
		{
			name: "email too long fails",
			modify: func(r *models.RegisterRequest) {
				r.Email = strings.Repeat("a", 250) + "@b.co"
			},
			wantErr:     true,
			errContains: "email",
		},
		{
			name:        "short password fails",
			modify:      func(r *models.RegisterRequest) { r.Password = "Short1" },
			wantErr:     true,
			errContains: "at least 8 characters",
		},
		{
			name:        "common password fails",
			modify:      func(r *models.RegisterRequest) { r.Password = "Password1" },
			wantErr:     true,
			errContains: "too common",
		},
		{
			name:        "no uppercase fails",
			modify:      func(r *models.RegisterRequest) { r.Password = "securepass1" },
			wantErr:     true,
			errContains: "uppercase",
		},
		{
			name:        "no lowercase fails",
			modify:      func(r *models.RegisterRequest) { r.Password = "SECUREPASS1" },
			wantErr:     true,
			errContains: "lowercase",
		},
		{
			name:        "no digit fails",
			modify:      func(r *models.RegisterRequest) { r.Password = "SecurePassX" },
			wantErr:     true,
			errContains: "digit",
		},
		{
			name: "password containing email user fails",
			modify: func(r *models.RegisterRequest) {
				r.Email = "johndoe@example.com"
				r.Password = "Johndoe1X"
			},
			wantErr:     true,
			errContains: "email address",
		},
		{
			name:        "invalid phone number fails",
			modify:      func(r *models.RegisterRequest) { r.PhoneNumber = "123456" },
			wantErr:     true,
			errContains: "E.164",
		},
		{
			name:        "empty first name fails",
			modify:      func(r *models.RegisterRequest) { r.FirstName = "" },
			wantErr:     true,
			errContains: "first name",
		},
		{
			name:        "first name with invalid chars fails",
			modify:      func(r *models.RegisterRequest) { r.FirstName = "John123" },
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "empty last name fails",
			modify:      func(r *models.RegisterRequest) { r.LastName = "" },
			wantErr:     true,
			errContains: "last name",
		},
		{
			name:        "invalid role fails",
			modify:      func(r *models.RegisterRequest) { r.Role = "admin" },
			wantErr:     true,
			errContains: "role",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := baseRegisterRequest()
			tt.modify(req)

			err := auth.ValidateRegisterRequest(req)

			if tt.wantErr {
				assert.Error(t, err)
				var appErr *common.AppError
				if assert.ErrorAs(t, err, &appErr) {
					assert.Contains(t, appErr.Message, tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLoginRequest(t *testing.T) {
	tests := []struct {
		name        string
		req         *models.LoginRequest
		wantErr     bool
		errContains string
	}{
		{
			name: "valid request passes",
			req: &models.LoginRequest{
				Email:    "user@example.com",
				Password: "anypassword",
			},
			wantErr: false,
		},
		{
			name: "invalid email fails",
			req: &models.LoginRequest{
				Email:    "bad-email",
				Password: "anypassword",
			},
			wantErr:     true,
			errContains: "invalid email",
		},
		{
			name: "empty password fails",
			req: &models.LoginRequest{
				Email:    "user@example.com",
				Password: "",
			},
			wantErr:     true,
			errContains: "password is required",
		},
		{
			name: "too long password fails",
			req: &models.LoginRequest{
				Email:    "user@example.com",
				Password: strings.Repeat("a", 129),
			},
			wantErr:     true,
			errContains: "password too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidateLoginRequest(tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				var appErr *common.AppError
				if assert.ErrorAs(t, err, &appErr) {
					assert.Contains(t, appErr.Message, tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
