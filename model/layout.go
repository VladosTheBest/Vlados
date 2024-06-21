package model

type Layout struct {
	ID      uint64 `json:"id"`
	OwnerID uint64 `json:"owner_id"`
	SortID  uint64 `json:"sort_id"`
	Name    string `json:"name"`
	Data    string `json:"data"`
}

type SortLayoutsRequest struct {
	ID     uint64 `json:"id"`
	SortID uint64 `json:"sort_id"`
}
