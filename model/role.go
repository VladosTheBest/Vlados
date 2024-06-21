package model

/*
 * Copyright © 2018-2019 Around25 SRL <office@around25.com>
 *
 * Licensed under the Around25 Wallet License Agreement (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.around25.com/licenses/EXCHANGE_LICENSE
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Cosmin Harangus <cosmin@around25.com>
 * @copyright 2018-2019 Around25 SRL <office@around25.com>
 * @license 	EXCHANGE_LICENSE
 */

import (
	"time"
)

type RoleAlias string

const (
	Member   RoleAlias = "member"
	Admin    RoleAlias = "admin"
	Business RoleAlias = "business"
	Broker   RoleAlias = "broker"
)

func (r RoleAlias) IsValid() bool {
	switch r {
	case Member, Admin, Business, Broker:
		return true
	}
	return false
}

func (r RoleAlias) IsBaseRole() bool {
	switch r {
	case Member, Admin, Business, Broker:
		return true
	}
	return false
}

// Role model
type Role struct {
	Alias       string       `gorm:"type:varchar(50);PRIMARY_KEY;" json:"alias"`
	Name        string       ` json:"name"`
	Scope       string       ` json:"-"`
	Permissions []Permission `gorm:"many2many:role_has_permissions;" json:"permissions"`
}

// RolePermission model
type RolePermission struct {
	RoleAlias       string `json:"role_alias"`
	PermissionAlias string `json:"permission_alias"`
}

// RoleWithStats model
type RoleWithStats struct {
	Alias            string    `json:"alias"`
	Name             string    `json:"name"`
	PermissionsCount int       `json:"permissions_count"`
	CreatedAt        time.Time `json:"created_at"`
}

// RoleWithStatsList model
type RoleWithStatsList struct {
	Roles []RoleWithStats
	Meta  PagingMeta
}

// NewRole creates a new Role
func NewRole(scope, alias, name string) *Role {
	return &Role{
		Scope: scope,
		Name:  name,
		Alias: alias,
	}
}

func (r RoleAlias) String() string {
	return string(r)
}
