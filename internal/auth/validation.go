package auth

import (
    "fmt"
    "regexp"
    "strings"
    "unicode"

    "github.com/richxcame/ride-hailing/pkg/common"
    "github.com/richxcame/ride-hailing/pkg/models"
    "github.com/richxcame/ride-hailing/pkg/validation"
)

var (
    // commonPasswords is a small set of extremely common passwords to reject
    commonPasswords = map[string]bool{
        "password":  true, "12345678": true, "password1": true,
        "qwerty123": true, "letmein1": true, "admin123":  true,
        "welcome1":  true, "monkey12":  true, "dragon12":  true,
        "master12":  true, "abc12345": true, "password123": true,
    }

    // nameRegex allows unicode letters, spaces, hyphens, and apostrophes
    nameRegex = regexp.MustCompile(`^[\p{L}][\p{L}\s'\-]{0,49}$`)
)

// ValidateRegisterRequest performs deep validation on registration input.
func ValidateRegisterRequest(req *models.RegisterRequest) error {
    // Email validation (beyond gin's basic check)
    if !validation.ValidateEmail(req.Email) {
        return common.NewBadRequestError("invalid email format", nil)
    }
    if len(req.Email) > 254 { // RFC 5321 max length
        return common.NewBadRequestError("email address too long", nil)
    }

    // Password strength validation
    if err := validatePasswordStrength(req.Password); err != nil {
        return err
    }

    // Check password doesn't contain email
    emailLower := strings.ToLower(req.Email)
    passwordLower := strings.ToLower(req.Password)
    emailUser := strings.Split(emailLower, "@")[0]
    if len(emailUser) > 3 && strings.Contains(passwordLower, emailUser) {
        return common.NewBadRequestError("password must not contain your email address", nil)
    }

    // Phone number validation (E.164)
    if !validation.ValidatePhoneNumber(req.PhoneNumber) {
        return common.NewBadRequestError("phone number must be in E.164 format (e.g., +1234567890)", nil)
    }

    // Name validation
    if err := validateName(req.FirstName, "first name"); err != nil {
        return err
    }
    if err := validateName(req.LastName, "last name"); err != nil {
        return err
    }

    // Role validation (defense in depth - gin already checks oneof)
    if req.Role != models.RoleRider && req.Role != models.RoleDriver {
        return common.NewBadRequestError("role must be 'rider' or 'driver'", nil)
    }

    return nil
}

// ValidateLoginRequest performs validation on login input.
func ValidateLoginRequest(req *models.LoginRequest) error {
    if !validation.ValidateEmail(req.Email) {
        return common.NewBadRequestError("invalid email format", nil)
    }
    if len(req.Password) == 0 {
        return common.NewBadRequestError("password is required", nil)
    }
    if len(req.Password) > 128 {
        return common.NewBadRequestError("password too long", nil)
    }
    return nil
}

// validatePasswordStrength enforces strong password requirements.
func validatePasswordStrength(password string) error {
    if len(password) < 8 {
        return common.NewBadRequestError("password must be at least 8 characters long", nil)
    }
    if len(password) > 128 {
        return common.NewBadRequestError("password must not exceed 128 characters", nil)
    }
    if commonPasswords[strings.ToLower(password)] {
        return common.NewBadRequestError("password is too common, please choose a stronger password", nil)
    }

    var hasUpper, hasLower, hasDigit bool
    for _, char := range password {
        switch {
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsDigit(char):
            hasDigit = true
        }
    }

    if !hasUpper {
        return common.NewBadRequestError("password must contain at least one uppercase letter", nil)
    }
    if !hasLower {
        return common.NewBadRequestError("password must contain at least one lowercase letter", nil)
    }
    if !hasDigit {
        return common.NewBadRequestError("password must contain at least one digit", nil)
    }

    return nil
}

// validateName validates a person's name.
func validateName(name, fieldName string) error {
    trimmed := strings.TrimSpace(name)
    if len(trimmed) == 0 {
        return common.NewBadRequestError(fmt.Sprintf("%s is required", fieldName), nil)
    }
    if len(trimmed) > 50 {
        return common.NewBadRequestError(fmt.Sprintf("%s must not exceed 50 characters", fieldName), nil)
    }
    if !nameRegex.MatchString(trimmed) {
        return common.NewBadRequestError(fmt.Sprintf("%s contains invalid characters", fieldName), nil)
    }
    return nil
}
