package service

import (
	"fmt"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetUserTransactions - get user transactions
func (service *Service) GetUserTransactions(userID uint64, limit, page, from, to int, market, transactionType, status, query string) (*model.TransactionListWithUser, error) {
	transactions := make([]model.TransactionWithUser, 0)
	var rowCount int64 = 0
	q := service.repo.ConnReader.Table("transactions as t").Where("t.user_id = ?", userID)

	if from > 0 {
		q = q.Where("t.created_at >= to_timestamp(?) ", from)
	}
	if to > 0 {
		q = q.Where("t.created_at <= to_timestamp(?) ", to)
	}
	if len(transactionType) > 0 {
		if transactionType == "any" {
			q = q.Where("t.tx_type IN ('deposit', 'withdraw')")
		} else {
			q = q.Where("t.tx_type = ? ", transactionType)
		}
	}
	if len(market) > 0 {
		q = q.Where("t.coin_symbol = ? ", market)
	}
	if len(status) > 0 && model.TxStatus(status).IsValid() {
		q = q.Where("t.status = ?", status)
	}
	if len(query) > 0 {
		squery := "%" + query + "%"
		q = q.Where("u.email LIKE ? OR t.txid LIKE ? OR t.address LIKE ?", squery, squery, squery)
	}

	dbc := q.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)

	db := q.Select("t.*, u.email as email, coins.blockchain_explorer").
		Joins("left join users as u on t.user_id = u.id").
		Joins("inner join coins on t.coin_symbol = coins.symbol").
		Order("created_at DESC")
	if limit == 0 {
		db = db.Find(&transactions)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&transactions)
	}

	transactionList := model.TransactionListWithUser{
		Transactions: transactions,
		Meta: model.PagingMeta{
			Page:   page,
			Count:  rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{})},
	}
	transactionList.Meta.Filter["status"] = "reports"

	return &transactionList, db.Error
}

// ExportUserTransactions
func (service *Service) ExportUserTransactions(orderID uint64, format, transactionType string, transactionData []model.TransactionWithUser) (*model.GeneratedFile, error) {
	data := [][]string{}
	data = append(data, []string{"ID", "Date & Time", "Currency", "Type", "Information", "Amount", "Status"})
	widths := []int{80, 45, 20, 20, 100, 50, 25}

	for i := 0; i < len(transactionData); i++ {
		o := transactionData[i]
		data = append(data, []string{
			fmt.Sprint(o.ID),
			o.CreatedAt.Format("2 Jan 2006 15:04:05"),
			fmt.Sprint(o.CoinSymbol),
			fmt.Sprint(o.TxType),
			fmt.Sprint(o.TxID),
			fmt.Sprintf("%f", o.Amount.V.Quantize(8)),
			fmt.Sprint(o.Status)})
	}

	var resp []byte
	var err error

	title := "Report"
	if transactionType == "reports" {
		title = "Transaction History Report"
	}
	if transactionType == "closed" {
		title = "Order History Report"
	}

	if format == "csv" {
		resp, err = CSVExport(data)
	} else {
		resp, err = PDFExport(data, widths, title)
	}

	generatedFile := model.GeneratedFile{
		Type:     format,
		DataType: transactionType,
		Data:     resp,
	}
	return &generatedFile, err
}
