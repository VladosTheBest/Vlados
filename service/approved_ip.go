package service

// HasApprovedIP godoc
func (service *Service) HasApprovedIP(userID uint64, ip string) (bool, error) {
	return service.repo.HasApprovedIP(userID, ip)
}

// AddPendingIPForUser godoc
func (service *Service) AddPendingIPForUser(userID uint64, ip string) error {
	return service.repo.AddPendingIPForUser(userID, ip)
}

// AddApprovedIPForUser godoc
func (service *Service) AddApprovedIPForUser(userID uint64, ip string) error {
	return service.repo.AddApprovedIPForUser(userID, ip)
}

// ApproveIPForUser godoc
func (service *Service) ApproveIPForUser(userID uint64, ip string) error {
	return service.repo.ApproveIPForUser(userID, ip)
}
