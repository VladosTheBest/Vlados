package service

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// ListChains - list all available chains
func (service *Service) ListChains() ([]model.Chain, error) {
	chains := make([]model.Chain, 0)
	db := service.repo.ConnReader.Find(&chains)
	return chains, db.Error
}

// ListChains - list all available chains
func (service *Service) ListChainsUnrestricted() ([]model.UnrestrictedChain, error) {
	chains := []model.UnrestrictedChain{}
	db := service.repo.Conn.
		Table("chains").
		Find(&chains)

	return chains, db.Error
}

// GetActiveChains - load the active chains from the database
func (service *Service) GetActiveChains() ([]model.Chain, error) {
	chains := make([]model.Chain, 0)
	db := service.repo.ConnReader.Where("status = 'active'").Find(&chains)
	return chains, db.Error
}

// GetChainsWithoutDepositAddresses - load chains with missing deposit addresses
func (service *Service) GetChainsWithoutDepositAddresses(userID uint64) ([]model.Chain, error) {
	chains := make([]model.Chain, 0)
	db := service.repo.Conn.
		Joins("LEFT JOIN addresses ON addresses.chain_symbol = chains.symbol AND addresses.user_id = ?", userID).
		Where("chains.status = 'active' AND addresses.id IS NULL").
		Find(&chains)
	return chains, db.Error
}

// GetChain - get a single chain by symbol
func (service *Service) GetChain(symbol string) (*model.Chain, error) {
	chain := model.Chain{}
	db := service.repo.Conn.Where("symbol = ?", symbol).First(&chain)
	return &chain, db.Error
}

// AddChain - add a new chain in the database
func (service *Service) AddChain(symbol string, name string, status model.ChainStatus) (*model.Chain, error) {
	chain := model.NewChain(symbol, name, status)
	err := service.repo.Create(chain)
	return chain, err
}

// UpdateChain - Update information about a chain
func (service *Service) UpdateChain(chain *model.Chain, name string, status model.ChainStatus) (*model.Chain, error) {
	chain.Name = name
	chain.Status = status
	err := service.repo.Update(chain)
	if err != nil {
		return nil, err
	}
	return chain, nil
}

// DeleteChain - removes a chain from the database
func (service *Service) DeleteChain(chain *model.Chain) error {
	chain.Status = model.ChainStatusInactive
	err := service.repo.Update(chain)
	return err
}

func (service *Service) GetChainsWithDepositAddresses(userID uint64, chainSymbol string) (*model.Chain, error) {
	var chain model.Chain
	db := service.repo.ConnReader.
		Joins("LEFT JOIN addresses ON addresses.chain_symbol = chains.symbol AND addresses.user_id = ?", userID).
		Where("chains.status = 'active' AND addresses.chain_symbol = ?", chainSymbol).
		First(&chain)
	return &chain, db.Error
}
