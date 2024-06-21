package service

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
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// NewPositionHandler create a new position handler
func (service *Service) NewPositionHandler() *TopicPositionHandler {
	return &TopicPositionHandler{
		repo:           service.repo,
		lastPositions:  map[string]int64{},
		appNameDefault: service.cfg.DatabaseCluster.Writer.ApplicationName,
	}
}

// TopicPositionHandler struct
type TopicPositionHandler struct {
	repo           *queries.Repo
	lastPositions  map[string]int64
	appNameDefault string
}

// Load latest offsets for the given topics
func (handler *TopicPositionHandler) Load(topics []string) (map[string]int64, error) {
	// by default start from offset -2
	positions := map[string]int64{}
	for _, topic := range topics {
		positions[topic] = -2
	}
	// load topics from the database
	queuePositions := []model.QueuePosition{}
	db := handler.repo.ConnReader.Where("topic IN (?) and component  = ?", topics, handler.appNameDefault).Find(&queuePositions)
	if db.Error != nil {
		return positions, db.Error
	}
	// update loaded partitions from db
	for _, queuePosition := range queuePositions {
		positions[queuePosition.Topic] = queuePosition.Offset
	}
	handler.lastPositions = positions
	return positions, nil
}

// Save topic offsets
func (handler *TopicPositionHandler) Save(positions map[string]int64) error {
	var err error
	for topic, offset := range positions {
		if lastPos, ok := handler.lastPositions[topic]; ok && lastPos == offset {
			continue
		}
		queuePosition := model.QueuePosition{Topic: topic, Component: handler.appNameDefault}
		db := handler.repo.Conn.
			Where(queuePosition).
			Assign(model.QueuePosition{Offset: offset}).
			FirstOrCreate(&queuePosition)
		if offset == 0 {
			queuePosition.Offset = 0
			db = handler.repo.Conn.Save(queuePosition)
		}
		if db.Error == nil {
			handler.lastPositions[topic] = offset
		} else {
			err = db.Error
		}
	}
	return err
}
