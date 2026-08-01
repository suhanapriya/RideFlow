package auth_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/richxcame/ride-hailing/internal/auth"
	"github.com/richxcame/ride-hailing/pkg/common"
	"github.com/richxcame/ride-hailing/pkg/models"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

// mockRepo implements auth.RepositoryInterface with configurable function fields.
type mockRepo struct {
	getUserByEmailFn func(ctx context.Context, email string) (*models.User, error)
	getUserByIDFn    func(ctx context.Context, id uuid.UUID) (*models.User, error)
	createUserFn     func(ctx context.Context, user *models.User) error
	createDriverFn   func(ctx context.Context, driver *models.Driver) error
	updateUserFn     func(ctx context.Context, user *models.User) error
}

func (m *mockRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return nil, errors.New("user not found")
}

func (m *mockRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.getUserByIDFn != nil {
		return m.getUserByIDFn(ctx, id)
	}
	return nil, errors.New("user not found")
}

func (m *mockRepo) CreateUser(ctx context.Context, user *models.User) error {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, user)
	}
	return nil
}

func (m *mockRepo) CreateDriver(ctx context.Context, driver *models.Driver) error {
	if m.createDriverFn != nil {
		return m.createDriverFn(ctx, driver)
	}
	return nil
}

func (m *mockRepo) UpdateUser(ctx context.Context, user *models.User) error {
	if m.updateUserFn != nil {
		return m.updateUserFn(ctx, user)
	}
	return nil
}

// validRegisterRequest returns a RegisterRequest that passes all validation.
func validRegisterRequest() *models.RegisterRequest {
	return &models.RegisterRequest{
		Email:       "test@example.com",
		Password:    "SecurePass1",
		PhoneNumber: "+1234567890",
		FirstName:   "John",
		LastName:    "Doe",
		Role:        models.RoleRider,
	}
}

func TestService_Register(t *testing.T) {
	tests := []struct {
		name        string
		req         *models.RegisterRequest
		repo        *mockRepo
		wantErr     bool
		errCode     int
		errContains string
	}{
		{
			name: "success as rider",
			req:  validRegisterRequest(),
			repo: &mockRepo{
				getUserByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
					return nil, errors.New("not found")
				},
				createUserFn: func(_ context.Context, _ *models.User) error {
					return nil
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate email conflict",
			req:  validRegisterRequest(),
			repo: &mockRepo{
				getUserByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
					return &models.User{Email: "test@example.com"}, nil
				},
			},
			wantErr:     true,
			errCode:     http.StatusConflict,
			errContains: "already exists",
		},
		{
			name: "repo CreateUser failure",
			req:  validRegisterRequest(),
			repo: &mockRepo{
				getUserByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
					return nil, errors.New("not found")
				},
				createUserFn: func(_ context.Context, _ *models.User) error {
					return errors.New("db connection failed")
				},
			},
			wantErr:     true,
			errCode:     http.StatusInternalServerError,
			errContains: "failed to create user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := auth.NewService(tt.repo, nil, 24)

			user, err := svc.Register(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				var appErr *common.AppError
				if assert.ErrorAs(t, err, &appErr) {
					assert.Equal(t, tt.errCode, appErr.Code)
					assert.Contains(t, appErr.Message, tt.errContains)
				}
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.req.Email, user.Email)
				assert.Equal(t, tt.req.Role, user.Role)
				assert.True(t, user.IsActive)
				assert.False(t, user.IsVerified)
				assert.Empty(t, user.PasswordHash, "password hash should be cleared in response")
			}
		})
	}
}

func TestService_Login(t *testing.T) {
	// Pre-hash a known password for login tests.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("SecurePass1"), bcrypt.DefaultCost)
	assert.NoError(t, err)

	tests := []struct {
		name        string
		req         *models.LoginRequest
		repo        *mockRepo
		wantErr     bool
		errCode     int
		errContains string
	}{
		{
			name: "user not found returns unauthorized",
			req: &models.LoginRequest{
				Email:    "noone@example.com",
				Password: "SecurePass1",
			},
			repo: &mockRepo{
				getUserByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
					return nil, errors.New("not found")
				},
			},
			wantErr:     true,
			errCode:     http.StatusUnauthorized,
			errContains: "invalid credentials",
		},
		{
			name: "wrong password returns unauthorized",
			req: &models.LoginRequest{
				Email:    "test@example.com",
				Password: "WrongPass1",
			},
			repo: &mockRepo{
				getUserByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
					return &models.User{
						ID:           uuid.New(),
						Email:        "test@example.com",
						PasswordHash: string(hashedPassword),
						IsActive:     true,
					}, nil
				},
			},
			wantErr:     true,
			errCode:     http.StatusUnauthorized,
			errContains: "invalid credentials",
		},
		{
			name: "inactive account returns unauthorized",
			req: &models.LoginRequest{
				Email:    "test@example.com",
				Password: "SecurePass1",
			},
			repo: &mockRepo{
				getUserByEmailFn: func(_ context.Context, _ string) (*models.User, error) {
					return &models.User{
						ID:           uuid.New(),
						Email:        "test@example.com",
						PasswordHash: string(hashedPassword),
						IsActive:     false,
					}, nil
				},
			},
			wantErr:     true,
			errCode:     http.StatusUnauthorized,
			errContains: "inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := auth.NewService(tt.repo, nil, 24)

			resp, err := svc.Login(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				var appErr *common.AppError
				if assert.ErrorAs(t, err, &appErr) {
					assert.Equal(t, tt.errCode, appErr.Code)
					assert.Contains(t, appErr.Message, tt.errContains)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestService_GetProfile(t *testing.T) {
	testID := uuid.New()

	tests := []struct {
		name        string
		userID      uuid.UUID
		repo        *mockRepo
		wantErr     bool
		errCode     int
		errContains string
	}{
		{
			name:   "success clears password hash",
			userID: testID,
			repo: &mockRepo{
				getUserByIDFn: func(_ context.Context, id uuid.UUID) (*models.User, error) {
					return &models.User{
						ID:           id,
						Email:        "test@example.com",
						PasswordHash: "somehash",
						FirstName:    "John",
						LastName:     "Doe",
					}, nil
				},
			},
			wantErr: false,
		},
		{
			name:   "user not found",
			userID: uuid.New(),
			repo: &mockRepo{
				getUserByIDFn: func(_ context.Context, _ uuid.UUID) (*models.User, error) {
					return nil, errors.New("not found")
				},
			},
			wantErr:     true,
			errCode:     http.StatusNotFound,
			errContains: "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := auth.NewService(tt.repo, nil, 24)

			user, err := svc.GetProfile(context.Background(), tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
				var appErr *common.AppError
				if assert.ErrorAs(t, err, &appErr) {
					assert.Equal(t, tt.errCode, appErr.Code)
					assert.Contains(t, appErr.Message, tt.errContains)
				}
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.userID, user.ID)
				assert.Empty(t, user.PasswordHash, "password hash should be cleared")
			}
		})
	}
}

func TestService_UpdateProfile(t *testing.T) {
	testID := uuid.New()
	profileImg := "https://example.com/img.png"

	tests := []struct {
		name        string
		userID      uuid.UUID
		updates     *models.User
		repo        *mockRepo
		wantErr     bool
		errCode     int
		errContains string
		checkFn     func(t *testing.T, user *models.User)
	}{
		{
			name:   "success updates fields",
			userID: testID,
			updates: &models.User{
				FirstName:    "Jane",
				LastName:     "Smith",
				PhoneNumber:  "+9876543210",
				ProfileImage: &profileImg,
			},
			repo: &mockRepo{
				getUserByIDFn: func(_ context.Context, id uuid.UUID) (*models.User, error) {
					return &models.User{
						ID:           id,
						Email:        "test@example.com",
						PasswordHash: "somehash",
						FirstName:    "John",
						LastName:     "Doe",
						PhoneNumber:  "+1234567890",
					}, nil
				},
				updateUserFn: func(_ context.Context, _ *models.User) error {
					return nil
				},
			},
			wantErr: false,
			checkFn: func(t *testing.T, user *models.User) {
				assert.Equal(t, "Jane", user.FirstName)
				assert.Equal(t, "Smith", user.LastName)
				assert.Equal(t, "+9876543210", user.PhoneNumber)
				assert.Equal(t, &profileImg, user.ProfileImage)
				assert.Empty(t, user.PasswordHash, "password hash should be cleared")
			},
		},
		{
			name:    "user not found",
			userID:  uuid.New(),
			updates: &models.User{FirstName: "Jane"},
			repo: &mockRepo{
				getUserByIDFn: func(_ context.Context, _ uuid.UUID) (*models.User, error) {
					return nil, errors.New("not found")
				},
			},
			wantErr:     true,
			errCode:     http.StatusNotFound,
			errContains: "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := auth.NewService(tt.repo, nil, 24)

			user, err := svc.UpdateProfile(context.Background(), tt.userID, tt.updates)

			if tt.wantErr {
				assert.Error(t, err)
				var appErr *common.AppError
				if assert.ErrorAs(t, err, &appErr) {
					assert.Equal(t, tt.errCode, appErr.Code)
					assert.Contains(t, appErr.Message, tt.errContains)
				}
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				if tt.checkFn != nil {
					tt.checkFn(t, user)
				}
			}
		})
	}
}
