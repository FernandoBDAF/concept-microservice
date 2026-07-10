package examples

import (
	"time"

	"github.com/FBDAF/microservices/services/common/models/domain"
)

// User represents a user in the system
type User struct {
	domain.VersionedBaseModel
	Username    string                 `json:"username" validate:"required,min=3,max=50" doc:"The user's username"`
	Email       string                 `json:"email" validate:"required,email" doc:"The user's email address"`
	Password    string                 `json:"-" validate:"required,min=8" doc:"The user's password (hashed)"`
	FirstName   string                 `json:"first_name" validate:"required" doc:"The user's first name"`
	LastName    string                 `json:"last_name" validate:"required" doc:"The user's last name"`
	Contact     domain.Contact         `json:"contact" validate:"required" doc:"The user's contact information"`
	Address     domain.Address         `json:"address" validate:"required" doc:"The user's address"`
	Roles       []string               `json:"roles" validate:"required,min=1" doc:"The user's roles"`
	Status      domain.Status          `json:"status" validate:"required" doc:"The user's current status"`
	Metadata    domain.MetadataList    `json:"metadata" doc:"Additional user metadata"`
	LastLogin   domain.CustomTime      `json:"last_login" doc:"The user's last login time"`
	Preferences map[string]interface{} `json:"preferences" doc:"User preferences"`
}

// NewUser creates a new User
func NewUser(username, email, password, firstName, lastName string) *User {
	now := time.Now()
	return &User{
		VersionedBaseModel: *domain.NewVersionedBaseModel(),
		Username:           username,
		Email:              email,
		Password:           password,
		FirstName:          firstName,
		LastName:           lastName,
		Status:             *domain.NewStatus("active", "User created"),
		LastLogin:          domain.CustomTime{Time: now},
		Preferences: map[string]interface{}{
			"theme":    "light",
			"language": "en",
		},
	}
}

// Validate validates the User
func (u *User) Validate() error {
	// Validate required fields
	if err := domain.ValidateString(u.Username, domain.ValidationRules{
		Required:  true,
		MinLength: 3,
		MaxLength: 50,
	}); err != nil {
		return err
	}

	if err := domain.ValidateString(u.Email, domain.ValidationRules{
		Required: true,
		Pattern:  domain.EmailPattern,
	}); err != nil {
		return err
	}

	if err := domain.ValidateString(u.Password, domain.ValidationRules{
		Required:  true,
		MinLength: 8,
	}); err != nil {
		return err
	}

	if err := domain.ValidateString(u.FirstName, domain.ValidationRules{
		Required: true,
	}); err != nil {
		return err
	}

	if err := domain.ValidateString(u.LastName, domain.ValidationRules{
		Required: true,
	}); err != nil {
		return err
	}

	// Validate nested structures
	if err := u.Contact.Validate(); err != nil {
		return err
	}

	if err := u.Address.Validate(); err != nil {
		return err
	}

	return nil
}

// GetFullName returns the user's full name
func (u *User) GetFullName() string {
	return u.FirstName + " " + u.LastName
}

// UpdateLastLogin updates the user's last login time
func (u *User) UpdateLastLogin() {
	u.LastLogin = domain.CustomTime{Time: time.Now()}
	u.VersionedBaseModel.SetUpdatedAt(time.Now())
}

// SetPreference sets a user preference
func (u *User) SetPreference(key string, value interface{}) {
	if u.Preferences == nil {
		u.Preferences = make(map[string]interface{})
	}
	u.Preferences[key] = value
	u.VersionedBaseModel.SetUpdatedAt(time.Now())
}

// GetPreference gets a user preference
func (u *User) GetPreference(key string) (interface{}, bool) {
	if u.Preferences == nil {
		return nil, false
	}
	value, exists := u.Preferences[key]
	return value, exists
}

// AddMetadata adds metadata to the user
func (u *User) AddMetadata(key string, value interface{}) {
	u.Metadata.Set(key, value)
	u.VersionedBaseModel.SetUpdatedAt(time.Now())
}

// GetMetadata gets metadata from the user
func (u *User) GetMetadata(key string) (interface{}, bool) {
	return u.Metadata.Get(key)
}

// RemoveMetadata removes metadata from the user
func (u *User) RemoveMetadata(key string) {
	u.Metadata.Delete(key)
	u.VersionedBaseModel.SetUpdatedAt(time.Now())
}

// UpdateStatus updates the user's status
func (u *User) UpdateStatus(value string, reason string) {
	u.Status = *domain.NewStatus(value, reason)
	u.VersionedBaseModel.SetUpdatedAt(time.Now())
}

// UserCreateRequest represents a request to create a user
type UserCreateRequest struct {
	Username    string                 `json:"username" validate:"required,min=3,max=50"`
	Email       string                 `json:"email" validate:"required,email"`
	Password    string                 `json:"password" validate:"required,min=8"`
	FirstName   string                 `json:"first_name" validate:"required"`
	LastName    string                 `json:"last_name" validate:"required"`
	Phone       string                 `json:"phone" validate:"required"`
	Address     domain.Address         `json:"address" validate:"required"`
	Roles       []string               `json:"roles" validate:"required,min=1"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
}

// UserUpdateRequest represents a request to update a user
type UserUpdateRequest struct {
	Username    string                 `json:"username,omitempty" validate:"omitempty,min=3,max=50"`
	Email       string                 `json:"email,omitempty" validate:"omitempty,email"`
	Password    string                 `json:"password,omitempty" validate:"omitempty,min=8"`
	FirstName   string                 `json:"first_name,omitempty"`
	LastName    string                 `json:"last_name,omitempty"`
	Phone       string                 `json:"phone,omitempty" validate:"omitempty"`
	Address     domain.Address         `json:"address,omitempty"`
	Roles       []string               `json:"roles,omitempty" validate:"omitempty,min=1"`
	Status      string                 `json:"status,omitempty"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
}

// UserResponse represents a user response
type UserResponse struct {
	ID          string                 `json:"id"`
	Username    string                 `json:"username"`
	Email       string                 `json:"email"`
	FirstName   string                 `json:"first_name"`
	LastName    string                 `json:"last_name"`
	Phone       string                 `json:"phone"`
	Address     domain.Address         `json:"address"`
	Roles       []string               `json:"roles"`
	Status      domain.Status          `json:"status"`
	LastLogin   *time.Time             `json:"last_login,omitempty"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// ToResponse converts a User to a UserResponse
func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		FirstName:   u.FirstName,
		LastName:    u.LastName,
		Phone:       u.Contact.Phone,
		Address:     u.Address,
		Roles:       u.Roles,
		Status:      u.Status,
		LastLogin:   &u.LastLogin.Time,
		Preferences: u.Preferences,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

// NewUserFromCreateRequest creates a User from a UserCreateRequest
func NewUserFromCreateRequest(req *UserCreateRequest) *User {
	user := NewUser(req.Username, req.Email, req.Password, req.FirstName, req.LastName)
	user.Contact = domain.Contact{
		Email: req.Email,
		Phone: req.Phone,
	}
	user.Address = req.Address
	user.Roles = req.Roles
	user.Preferences = req.Preferences
	return user
}

// Update updates a User from a UserUpdateRequest
func (u *User) Update(req *UserUpdateRequest) {
	if req.Username != "" {
		u.Username = req.Username
	}
	if req.Email != "" {
		u.Email = req.Email
		u.Contact.Email = req.Email
	}
	if req.Password != "" {
		u.Password = req.Password
	}
	if req.FirstName != "" {
		u.FirstName = req.FirstName
	}
	if req.LastName != "" {
		u.LastName = req.LastName
	}
	if req.Phone != "" {
		u.Contact.Phone = req.Phone
	}
	if req.Address.Street != "" {
		u.Address = req.Address
	}
	if len(req.Roles) > 0 {
		u.Roles = req.Roles
	}
	if req.Status != "" {
		u.UpdateStatus(req.Status, "User updated")
	}
	if req.Preferences != nil {
		u.Preferences = req.Preferences
	}
	u.UpdatedAt = time.Now()
}

/*
Example usage:

func main() {
	// Create a new user
	user := NewUser(
		"johndoe",
		"john.doe@example.com",
		"securepassword123",
		"John",
		"Doe",
	)

	// Set contact information
	user.Contact = domain.Contact{
		Email: "john.doe@example.com",
		Phone: "+1234567890",
	}

	// Set address
	user.Address = domain.Address{
		Street:     "123 Main St",
		City:       "New York",
		State:      "NY",
		PostalCode: "10001",
		Country:    "USA",
	}

	// Add metadata
	user.AddMetadata("role", "admin")
	user.AddMetadata("department", "engineering")

	// Set preferences
	user.SetPreference("theme", "dark")
	user.SetPreference("notifications", true)

	// Update status
	user.UpdateStatus("active", "User activated")

	// Validate user
	if err := user.Validate(); err != nil {
		log.Fatalf("User validation failed: %v", err)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(user)
	if err != nil {
		log.Fatalf("Failed to marshal user: %v", err)
	}

	fmt.Printf("User JSON: %s\n", string(jsonData))

	// Example of migration
	registry := domain.NewMigrationRegistry()
	registry.RegisterMigration(reflect.TypeOf(user), domain.Version{Major: 1, Minor: 0, Patch: 0}, func(old interface{}) (interface{}, error) {
		u, ok := old.(*User)
		if !ok {
			return nil, errors.New("invalid type for migration")
		}
		// Example migration logic
		u.VersionedBaseModel.SetVersion(domain.Version{Major: 2, Minor: 0, Patch: 0})
		return u, nil
	})

	migrated, err := registry.Migrate(user, domain.Version{Major: 2, Minor: 0, Patch: 0})
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	migratedUser := migrated.(*User)
	fmt.Printf("Migrated User: %+v\n", migratedUser)
}
*/
