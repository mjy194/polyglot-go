package data

import (
	"context"
	"errors"

	"polyglot/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IdentityRepository persists users, roles, sessions, and inbound API keys.
type IdentityRepository interface {
	UpsertUser(ctx context.Context, user domain.User) (domain.User, error)
	GetUser(ctx context.Context, id string) (domain.User, bool, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, bool, error)
	ListUsers(ctx context.Context) ([]domain.User, error)
	CountUsers(ctx context.Context) (int64, error)
	CountPasswordUsers(ctx context.Context) (int64, error)

	UpsertRole(ctx context.Context, role domain.Role) (domain.Role, error)
	GetRole(ctx context.Context, id string) (domain.Role, bool, error)
	ListRoles(ctx context.Context) ([]domain.Role, error)
	AssignRole(ctx context.Context, userRole domain.UserRole) error
	RemoveRole(ctx context.Context, userID, roleID string) error
	ListUserRoles(ctx context.Context, userID string) ([]domain.UserRole, error)

	UpsertAPIKey(ctx context.Context, apiKey domain.APIKey) (domain.APIKey, error)
	GetAPIKeyByKey(ctx context.Context, key string) (domain.APIKey, bool, error)
	ListAPIKeys(ctx context.Context) ([]domain.APIKey, error)
	ListAPIKeysByUser(ctx context.Context, userID string) ([]domain.APIKey, error)
	DeleteAPIKey(ctx context.Context, id string) error
	CountAPIKeys(ctx context.Context) (int64, error)

	CreateAdminSession(ctx context.Context, session domain.AdminSession) (domain.AdminSession, error)
	GetAdminSessionByTokenHash(ctx context.Context, tokenHash string) (domain.AdminSession, bool, error)
	RevokeAdminSessionByTokenHash(ctx context.Context, tokenHash string) error
}

type gormIdentityRepository struct {
	db *gorm.DB
}

func NewGormIdentityRepository(db *gorm.DB) IdentityRepository {
	return &gormIdentityRepository{db: db}
}

func (r *gormIdentityRepository) UpsertUser(ctx context.Context, user domain.User) (domain.User, error) {
	if user.ID == "" {
		user.ID = newID(idPrefixUser)
	}
	record := userToRecord(user)
	updateColumns := []string{
		"email",
		"display_name",
		"status",
		"group_name",
		"last_login_at",
		"updated_at",
	}
	if user.PasswordHash != "" {
		updateColumns = append(updateColumns, "password_hash")
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(&record).Error
	return userFromRecord(record), err
}

func (r *gormIdentityRepository) GetUser(ctx context.Context, id string) (domain.User, bool, error) {
	var record UserRecord
	err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, false, nil
	}
	if err != nil {
		return domain.User{}, false, err
	}
	return userFromRecord(record), true, nil
}

func (r *gormIdentityRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	var record UserRecord
	err := r.db.WithContext(ctx).First(&record, "email = ?", email).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, false, nil
	}
	if err != nil {
		return domain.User{}, false, err
	}
	return userFromRecord(record), true, nil
}

func (r *gormIdentityRepository) ListUsers(ctx context.Context) ([]domain.User, error) {
	var records []UserRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&records).Error
	return usersFromRecords(records), err
}

func (r *gormIdentityRepository) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&UserRecord{}).Count(&count).Error
	return count, err
}

func (r *gormIdentityRepository) CountPasswordUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&UserRecord{}).
		Where("password_hash <> ''").
		Count(&count).Error
	return count, err
}

func (r *gormIdentityRepository) UpsertRole(ctx context.Context, role domain.Role) (domain.Role, error) {
	if role.ID == "" {
		role.ID = newID(idPrefixRole)
	}
	record := roleToRecord(role)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"description",
			"permissions",
			"updated_at",
		}),
	}).Create(&record).Error
	return roleFromRecord(record), err
}

func (r *gormIdentityRepository) GetRole(ctx context.Context, id string) (domain.Role, bool, error) {
	var record RoleRecord
	err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Role{}, false, nil
	}
	if err != nil {
		return domain.Role{}, false, err
	}
	return roleFromRecord(record), true, nil
}

func (r *gormIdentityRepository) ListRoles(ctx context.Context) ([]domain.Role, error) {
	var records []RoleRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&records).Error
	return rolesFromRecords(records), err
}

func (r *gormIdentityRepository) AssignRole(ctx context.Context, userRole domain.UserRole) error {
	record := userRoleToRecord(userRole)
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "role_id"}},
		DoNothing: true,
	}).Create(&record).Error
}

func (r *gormIdentityRepository) RemoveRole(ctx context.Context, userID, roleID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Delete(&UserRoleRecord{}).Error
}

func (r *gormIdentityRepository) ListUserRoles(ctx context.Context, userID string) ([]domain.UserRole, error) {
	var records []UserRoleRecord
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&records).Error
	return userRolesFromRecords(records), err
}

func (r *gormIdentityRepository) UpsertAPIKey(ctx context.Context, apiKey domain.APIKey) (domain.APIKey, error) {
	if apiKey.ID == "" {
		apiKey.ID = newID(idPrefixAPIKey)
	}
	record := apiKeyToRecord(apiKey)
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id",
			"name",
			"key",
			"scopes",
			"status",
			"group_name",
			"expires_at",
			"last_used_at",
			"updated_at",
		}),
	}).Create(&record).Error
	return apiKeyFromRecord(record), err
}

func (r *gormIdentityRepository) GetAPIKeyByKey(ctx context.Context, key string) (domain.APIKey, bool, error) {
	var record APIKeyRecord
	err := r.db.WithContext(ctx).First(&record, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.APIKey{}, false, nil
	}
	if err != nil {
		return domain.APIKey{}, false, err
	}
	return apiKeyFromRecord(record), true, nil
}

func (r *gormIdentityRepository) ListAPIKeys(ctx context.Context) ([]domain.APIKey, error) {
	var records []APIKeyRecord
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&records).Error
	return apiKeysFromRecords(records), err
}

func (r *gormIdentityRepository) ListAPIKeysByUser(ctx context.Context, userID string) ([]domain.APIKey, error) {
	var records []APIKeyRecord
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&records).Error
	return apiKeysFromRecords(records), err
}

func (r *gormIdentityRepository) DeleteAPIKey(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&APIKeyRecord{}).Error
}

func (r *gormIdentityRepository) CountAPIKeys(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&APIKeyRecord{}).Count(&count).Error
	return count, err
}

func (r *gormIdentityRepository) CreateAdminSession(ctx context.Context, session domain.AdminSession) (domain.AdminSession, error) {
	if session.ID == "" {
		session.ID = newID(idPrefixAdminSession)
	}
	record := adminSessionToRecord(session)
	err := r.db.WithContext(ctx).Create(&record).Error
	return adminSessionFromRecord(record), err
}

func (r *gormIdentityRepository) GetAdminSessionByTokenHash(ctx context.Context, tokenHash string) (domain.AdminSession, bool, error) {
	var record AdminSessionRecord
	err := r.db.WithContext(ctx).First(&record, "token_hash = ?", tokenHash).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.AdminSession{}, false, nil
	}
	if err != nil {
		return domain.AdminSession{}, false, err
	}
	return adminSessionFromRecord(record), true, nil
}

func (r *gormIdentityRepository) RevokeAdminSessionByTokenHash(ctx context.Context, tokenHash string) error {
	return r.db.WithContext(ctx).
		Model(&AdminSessionRecord{}).
		Where("token_hash = ?", tokenHash).
		Updates(map[string]interface{}{"status": domain.StatusDisabled}).Error
}
