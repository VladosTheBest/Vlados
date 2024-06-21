package service

import (
	"fmt"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"github.com/lib/pq"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

func (service *Service) GetDistributionsForUser(userID uint64, limit, page, from, to int) (*model.LiabilityList, error) {
	liabilities := make([]model.Liability, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader.Where("ref_type = 'distribution' AND user_id = ?", userID)

	if from > 0 {
		q = q.Where("created_at >= to_timestamp(?) ", from)
	}

	if to > 0 {
		q = q.Where("created_at <= to_timestamp(?) ", to)
	}

	dbc := q.Table("liabilities").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	q = q.Order("id DESC")
	if limit != 0 {
		q = q.Limit(limit).Offset((page - 1) * limit)
	}
	db := q.Select("*").Group("id").Find(&liabilities)

	liabilityList := model.LiabilityList{
		Liabilities: liabilities,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &liabilityList, db.Error
}

// GetDistributedBonus returns the number of distributed bonus (in PRDX) from the system
func (service *Service) GetDistributedBonus() (*decimal.Big, error) {
	result := decimal.New(0, 1)
	coins := []string{"prdx", "terc"}
	rows, err := service.repo.Conn.
		Table("manual_distribution_funds").
		Select("SUM(converted_balance) as total").
		Joins("inner join manual_distributions on manual_distribution_funds.distribution_id = manual_distributions.id").
		Where("manual_distribution_funds.status = ?", "completed").
		Where("manual_distribution_funds.converted_coin_symbol = ANY(?)", pq.Array(coins)).Rows()

	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		totalBalance := &postgres.Decimal{}
		_ = rows.Scan(&totalBalance)
		if totalBalance == nil {
			return result, err
		}
		result = totalBalance.V
	}
	return result, err
}

// ExportDistributionsForUser  - distribution data to export
func (service *Service) ExportDistributionsForUser(format string, liabilities []model.Liability) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "Date & Time", "Distribution ID", "Amount"})
	widths := []int{15, 45, 45, 45}

	for i := 0; i < len(liabilities); i++ {
		o := liabilities[i]
		data = append(data, []string{fmt.Sprint(o.ID), o.CreatedAt.Format("2 Jan 2006 15:04:05"), fmt.Sprint(o.RefID), fmt.Sprint(utils.Fmt(o.Credit.V))})
	}

	var resp []byte
	var err error

	title := "Distributions Report"

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: "distributions",
		Data:     resp,
	}
	return &generatedFile, err
}

// GetDistributionEvents godoc
func (service *Service) GetDistributionEvents(limit, page int) (*model.DistributionList, error) {
	distributions := make([]model.Distribution, 0)
	var rowCount int64 = 0

	q := service.repo.ConnReader

	dbc := q.Table("distributions").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db := q.Order("id DESC")
	if limit == 0 {
		db = db.Find(&distributions)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&distributions)
	}

	distributionList := model.DistributionList{
		Distributions: distributions,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &distributionList, db.Error
}

// GetDistributionOrders godoc
func (service *Service) GetDistributionOrders(distribution_id string, limit, page int) (*model.DistributionOrderList, error) {
	distributionOrders := make([]model.DistributionOrder, 0)
	var rowCount int64 = 0

	q := service.repo.Conn.Where("ref_id = ?", distribution_id)
	dbc := q.Table("distributions").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	q = q.Order("id DESC")
	if limit != 0 {
		q = q.Limit(limit).Offset((page - 1) * limit)
	}
	db := q.Find(&distributionOrders)

	distributionOrderList := model.DistributionOrderList{
		DistributionOrders: distributionOrders,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &distributionOrderList, db.Error
}

// GetDistributionEntries godoc
func (service *Service) GetDistributionEntries(distribution_id string, limit, page int) (*model.LiabilityList, error) {
	liabilities := make([]model.Liability, 0)
	var rowCount int64 = 0

	q := service.repo.Conn.Where("ref_type = 'distribution' AND ref_id = ?", distribution_id)

	dbc := q.Table("liabilities").Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	q = q.Order("id DESC")
	if limit != 0 {
		q = q.Limit(limit).Offset((page - 1) * limit)
	}
	db := q.Find(&liabilities)

	liabilityList := model.LiabilityList{
		Liabilities: liabilities,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}

	return &liabilityList, db.Error
}
