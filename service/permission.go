package service

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetRole function
func (s *Service) GetRole(id uint) (*model.Role, error) {
	role := model.Role{}
	db := s.repo.Conn.Where("id = ?", id).First(&role)
	return &role, db.Error
}

// GetRolesByScope - get rolws for specified scope
func (s *Service) GetRolesByScope(scope string) ([]model.Role, error) {
	roles := make([]model.Role, 0)

	db := s.repo.ConnReader.Where("scope = ?", scope).Find(&roles)
	return roles, db.Error
}

// ValidateRoleInScope - check if the given role exists
func (s *Service) ValidateRoleInScope(role, scope string) bool {
	roles := model.Role{}
	db := s.repo.ConnReader.Where("alias = ? AND scope = ?", role, scope).Find(&roles)
	return !(db.Error != nil)
}

// GetRoleByAlias function
func (s *Service) GetRoleByAlias(alias string) (*model.Role, error) {
	role := model.Role{}
	db := s.repo.Conn.Where("alias = ?", alias).First(&role)
	return &role, db.Error
}

// GetPermissionsByRoleID function
func (s *Service) GetPermissionsByRoleID(roleID uint) ([]model.Permission, error) {
	permissions := make([]model.Permission, 0)
	db := s.repo.ConnReader.Joins("left join role_has_permissions on role_has_permissions.permission_id = permissions.id").Where("role_has_permissions.role_id = ?", roleID).Find(&permissions)

	return permissions, db.Error
}

// GetPermissionsByRoleAlias function
func (s *Service) GetPermissionsByRoleAlias(alias string) ([]model.Permission, error) {
	permissions := make([]model.Permission, 0)
	db := s.repo.ConnReader.Joins("left join role_has_permissions on role_has_permissions.permission_alias = permissions.alias").Where("role_has_permissions.role_alias = ?", alias).Find(&permissions)
	return permissions, db.Error
}

// GetPermissionAliasesByRoleAlias function
func (s *Service) GetPermissionAliasesByRoleAlias(alias string) (map[string]bool, error) {
	return GetPermissionAliases(s.GetPermissionsByRoleAlias(alias))
}

// GetPermissionAliases returns the list of the permission aliases
func GetPermissionAliases(perms []model.Permission, err error) (map[string]bool, error) {
	aliases := make(map[string]bool)
	if err != nil {
		return aliases, err
	}
	for _, perm := range perms {
		aliases[perm.Alias] = true
	}
	return aliases, nil
}

// GetUserRoleWithPermissions - get role with permissions
func (s *Service) GetUserRoleWithPermissions(alias string) (*model.Role, error) {
	role := model.Role{}
	db := s.repo.Conn.
		Set("gorm:auto_preload", true).
		Where("roles.alias = ?", alias).
		First(&role)

	return &role, db.Error
}

// GetPermissions function
func (s *Service) GetPermissions() ([]model.Permission, error) {
	permissions := make([]model.Permission, 0)
	db := s.repo.ConnReader.
		Order("permissions.alias ASC").
		Find(&permissions)

	return permissions, db.Error
}

func (s *Service) ValidateRegistrationRole(role model.RoleAlias) model.RoleAlias {
	switch role {
	case model.Broker:
	case model.Business:
	case model.Member:
		return role
	}

	return model.Member
}
