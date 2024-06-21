package service

import (
	"errors"
	"strings"

	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (service *Service) GetLayouts(userID uint64) ([]model.Layout, int64, error) {
	result, count, err := service.repo.GetLayouts(userID)
	if err != nil {
		log.Error().Err(err).
			Uint64("user_id", userID).
			Msg("unable to get users layouts")
		return nil, 0, err
	}

	return result, count, nil
}

func (service *Service) SaveLayout(layout *model.Layout) (*model.Layout, error) {

	if strings.TrimSpace(layout.Name) == "" {
		return nil, errors.New("name should be filled")
	}

	if strings.TrimSpace(layout.Data) == "" {
		return nil, errors.New("data should be filled")
	}

	usersLayouts, _, err := service.repo.GetLayouts(layout.OwnerID)
	if err != nil {
		log.Error().Err(err).
			Uint64("user_id", layout.OwnerID).
			Str("layout name", layout.Name).
			Msg("unable to get users layouts")
		return nil, err
	}

	layout.SortID = service.getLastLayoutsSortID(usersLayouts) + 1

	err = service.repo.SaveLayout(layout)
	if err != nil {
		log.Error().Err(err).
			Uint64("user_id", layout.OwnerID).
			Str("layout name", layout.Name).
			Msg("unable to save layout in db")
		return nil, err
	}

	return layout, nil
}

func (service *Service) DeleteLayout(userID, layoutID uint64) error {

	if layoutID == 0 {
		return errors.New("id of layout should be filled")
	}

	if err := service.repo.DeleteLayout(layoutID); err != nil {
		log.Error().Err(err).
			Uint64("user_id", userID).
			Uint64("layout_id", layoutID).
			Msg("unable to save layout in db")
		return err
	}

	return nil
}

func (service *Service) UpdateLayout(layout model.Layout) error {

	if layout.ID == 0 {
		return errors.New("id of layout should be filled")
	}

	if layout.Data != "" {
		if err := service.repo.UpdateLayoutData(layout.ID, layout.Data); err != nil {
			log.Error().Err(err).
				Uint64("layout_id", layout.ID).
				Msg("unable to update layout.data in db")
			return err
		}
	}

	if layout.Name != "" {
		if err := service.repo.UpdateLayoutName(layout.ID, layout.Name); err != nil {
			log.Error().Err(err).
				Uint64("layout_id", layout.ID).
				Msg("unable to update layout.name in db")
			return err
		}
	}

	return nil
}

func (service *Service) SortLayouts(data []model.SortLayoutsRequest) error {

	if len(data) < 2 {
		return errors.New("not enough data")
	}

	for _, v := range data {
		if err := service.repo.UpdateLayoutSortID(v.ID, v.SortID); err != nil {
			log.Error().Err(err).
				Uint64("layout_id", v.ID).
				Msg("unable to update layout.sort_id in db")
			return err
		}
	}

	return nil
}

func (service *Service) getLastLayoutsSortID(layouts []model.Layout) uint64 {
	if len(layouts) == 0 {
		return 0
	}

	last := layouts[0].SortID
	for _, v := range layouts {
		if v.SortID > last {
			last = v.SortID
		}
	}

	return last
}
