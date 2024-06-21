package service

import (
	"errors"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gorm.io/gorm"
	"strings"
	// "github.com/ericlagergren/decimal/sql/postgres"
	// "github.com/jinzhu/gorm"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetTopInviters
func (service *Service) GetTopInviters() ([]model.TopInviters, error) {
	users := make([]model.TopInviters, 0)
	limit := 10
	q := service.repo.ConnReader.
		Table("users").
		Select("count(referrals.user_id) as level1_invited, users.created_at, CONCAT (LEFT(users.email,3), '****',  RIGHT(users.email,3)) as email").
		Joins("inner join referrals on users.id = referrals.level_1_id").
		Order("count(referrals.user_id) DESC").
		Group("users.id").
		Limit(limit).
		Find(&users)
	if q.Error != nil {
		return users, q.Error
	}
	return users, nil
}

// GetReferrals
func (service *Service) GetReferrals(userID uint64, limit, page int) (*model.ReferralEarningsResponse, error) {
	data := []model.ReferralEarningsResponseData{}
	var rowCount int64 = 0

	db := service.repo.ConnReader.Table("users u").
		Joins("left join users u1 ON u1.referral_id = u.referral_code").
		Where("u.id = ?", userID).
		Where("u1.id is not null")

	dbc := db.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	selectList := []string{
		"CONCAT (LEFT(u1.email,3), '****',  RIGHT(u1.email,3)) as email",
		"u1.created_at as register_date",
		"referrals_count_earnings(u.id, array_agg(distinct u1.id)) as l1_earnings",
		"referrals_count_earnings(u.id, array_agg(distinct u2.id)) as l2_earnings",
		"referrals_count_earnings(u.id, array_agg(distinct u3.id)) as l3_earnings",
		"count(distinct u2.id)::bigint as l2_users",
		"count(distinct u3.id)::bigint as l3_users",
	}
	db = db.
		Joins("left join users u2 ON u2.referral_id = u1.referral_code").
		Joins("left join users u3 ON u3.referral_id = u2.referral_code").
		Group("u.id, u1.id").
		Select(strings.Join(selectList, ",")).
		Order("register_date DESC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&data)
	if db.Error != nil {
		return nil, db.Error
	}

	response := model.ReferralEarningsResponse{
		Data: data,
		Meta: model.PagingMeta{
			Page:   int(page),
			Count:  rowCount,
			Limit:  int(limit),
			Filter: make(map[string]interface{})},
	}
	return &response, db.Error
}

// GetReferralsWithEarnings
func (service *Service) GetReferralsWithEarnings(userID uint64, limit, page int, query string) (*model.UserWithReferralsList, error) {
	users := make([]model.UserWithReferrals, 0)
	usersList := model.UserWithReferralsList{}
	var rowCount int64 = 0

	q := service.repo.ConnReader.Table("users u").
		Joins("left join users u1 ON u1.referral_id = u.referral_code").
		Where("u1.id is not null")

	if userID != 0 {
		q = q.Where("u.id = ?", userID)
		q = q.Select("count(u1.id) as level1_invited, referrals_count_earnings(u.id, array_agg(distinct u1.id)) as earned, u1.*")
		q = q.Group("u.id, u1.id")
	} else {
		q = q.Select("DISTINCT count(u.id) as level1_invited, referrals_count_earnings(u.id, array_agg(distinct u1.id)) as earned, u.*")
		q = q.Group("u.id")
	}
	//
	if len(query) > 0 {
		query_param := "%" + query + "%"
		if userID != 0 {
			q = q.Where("u1.email LIKE ?", query_param)
		} else {
			q = q.Where("u.email LIKE ?", query_param)
		}
	}

	q = q.Session(&gorm.Session{}) // gorm session is used to execute multiple queries below using the base query above
	res := q.Order("level1_invited DESC").
		Limit(limit).
		Offset((page - 1) * limit).
		Find(&users)

	if res.Error != nil {
		return &usersList, res.Error
	}

	var dbc *gorm.DB
	if userID != 0 {
		dbc = q.Select("count(DISTINCT u1.id) as level1_invited")
	} else {
		dbc = q.Select("count(DISTINCT u.id) as level1_invited")
	}

	dbc = dbc.Count(&rowCount)
	if dbc.Error != nil {
		return &usersList, dbc.Error
	}

	usersList = model.UserWithReferralsList{
		Users: users,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &usersList, nil
}

func (service *Service) GetReferralEarningsTotalAll() (*decimal.Big, error) {
	data := &struct{ Balance *postgres.Decimal }{Balance: &postgres.Decimal{V: new(decimal.Big)}}

	db := service.repo.ConnReader.
		Table("referral_earnings").
		Select("sum(amount) as balance").
		Scan(data)
	if db.Error != nil {
		if errors.Is(db.Error, gorm.ErrRecordNotFound) {
			return new(decimal.Big), nil
		}
		return nil, db.Error
	}
	if data.Balance != nil && data.Balance.V != nil {
		return data.Balance.V, nil
	}

	return new(decimal.Big), nil
}

func (service *Service) GetReferralEarningsTotalAllByUser(userID uint64) (*decimal.Big, error) {
	data := &struct{ Balance *postgres.Decimal }{Balance: &postgres.Decimal{V: new(decimal.Big)}}

	db := service.repo.ConnReader.
		Table("referral_earnings").
		Select("sum(amount) as balance").
		Where("user_id = ?", userID).
		Scan(data)
	if db.Error != nil {
		if errors.Is(db.Error, gorm.ErrRecordNotFound) {
			return new(decimal.Big), nil
		}
		return nil, db.Error
	}
	if data.Balance != nil && data.Balance.V != nil {
		return data.Balance.V, nil
	}

	return new(decimal.Big), nil
}

// GetReferralEarningsTotal - return total earnings
func (service *Service) GetReferralEarningsTotal(userID uint64) (*decimal.Big, error) {
	return new(decimal.Big), nil
	// data := &struct{ BalanceView *postgres.Decimal }{BalanceView: &postgres.Decimal{V: new(decimal.Big)}}
	// db := service.repo.Conn.
	// 	Table("liabilities").
	// 	Select("sum(credit - debit) as balance").
	// 	Where("user_id = ? AND ref_type = 'referral'", userID).
	// 	Scan(data)
	// if db.Error != nil {
	// 	if gorm.IsRecordNotFoundError(db.Error) {
	// 		return new(decimal.Big), nil
	// 	}
	// 	return nil, db.Error
	// }
	// if data.BalanceView != nil && data.BalanceView.V != nil {
	// 	return data.BalanceView.V, nil
	// }
	// return new(decimal.Big), nil
}
