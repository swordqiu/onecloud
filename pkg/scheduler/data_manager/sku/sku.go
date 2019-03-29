// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sku

import (
	"fmt"
	"sync"
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/util/wait"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/compute/models"
)

var (
	skuManager *SSkuManager
)

func Start(refreshInterval time.Duration) {
	skuManager = &SSkuManager{
		skuMap:          newSkuMap(),
		refreshInterval: refreshInterval,
	}
	skuManager.sync()
}

func GetByZone(instanceType, zoneId string) *models.SServerSku {
	return skuManager.GetByZone(instanceType, zoneId)
}

type skuMap struct {
	*sync.Map
}

type skuList []*models.SServerSku

func (l skuList) Has(newSku *models.SServerSku) (int, bool) {
	for i, oldSku := range l {
		if oldSku.Id == newSku.Id {
			return i, true
		}
	}
	return -1, false
}

func (l skuList) DebugString() string {
	return fmt.Sprintf("%s", jsonutils.Marshal(l).String())
}

func (l skuList) GetByZone(zoneId string) *models.SServerSku {
	for _, s := range l {
		if s.ZoneId == zoneId {
			return s
		}
	}
	return nil
}

func newSkuMap() *skuMap {
	return &skuMap{
		Map: new(sync.Map),
	}
}

func (cache *skuMap) Get(instanceType string) skuList {
	value, ok := cache.Load(instanceType)
	if ok {
		return value.(skuList)
	}
	return nil
}

func (cache *skuMap) Add(instanceType string, sku *models.SServerSku) {
	skus := cache.Get(instanceType)
	if skus == nil {
		skus = make([]*models.SServerSku, 0)
	}
	skus = append(skus, sku)
	cache.Store(instanceType, skus)
}

type SSkuManager struct {
	// skus cache all server skus in database, key is InstanceType, value is []models.SServerSku
	skuMap          *skuMap
	refreshInterval time.Duration
}

func (m *SSkuManager) syncOnce() {
	log.Infof("SkuManager start sync")
	startTime := time.Now()

	skus := make([]models.SServerSku, 0)
	q := models.ServerSkuManager.Query()
	q = q.Filter(
		sqlchemy.OR(
			sqlchemy.Equals(q.Field("prepaid_status"), models.SkuStatusAvailable),
			sqlchemy.Equals(q.Field("postpaid_status"), models.SkuStatusAvailable)))
	if err := q.All(&skus); err != nil {
		log.Errorf("SkuManager query all available skus error: %v", err)
		return
	}
	m.skuMap = newSkuMap()
	for _, sku := range skus {
		tmp := sku
		m.skuMap.Add(sku.Name, &tmp)
	}
	log.Infof("SkuManager end sync, consume %s", time.Since(startTime))
}

func (m *SSkuManager) sync() {
	wait.Forever(m.syncOnce, m.refreshInterval)
}

func (m *SSkuManager) GetByZone(instanceType, zoneId string) *models.SServerSku {
	l := m.skuMap.Get(instanceType)
	if l == nil {
		return nil
	}
	return l.GetByZone(zoneId)
}
