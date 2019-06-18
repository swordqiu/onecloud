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

package models

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"yunion.io/x/jsonutils"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/appctx"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
)

type SManagedResourceBase struct {
	ManagerId string `width:"128" charset:"ascii" nullable:"true" list:"admin" create:"optional"` // Column(VARCHAR(ID_LENGTH, charset='ascii'), nullable=True)
}

func (self *SManagedResourceBase) GetCloudprovider() *SCloudprovider {
	if len(self.ManagerId) > 0 {
		return CloudproviderManager.FetchCloudproviderById(self.ManagerId)
	}
	return nil
}

func (self *SManagedResourceBase) GetCloudaccount() *SCloudaccount {
	cp := self.GetCloudprovider()
	if cp == nil {
		return nil
	}
	return cp.GetCloudaccount()
}

func (self *SManagedResourceBase) GetCustomizeColumns(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) *jsonutils.JSONDict {
	provider := self.GetCloudprovider()
	if provider == nil {
		return nil
	}
	info := map[string]string{
		"manager":    provider.GetName(),
		"manager_id": provider.GetId(),
		"provider":   provider.Provider,
	}
	if len(provider.ProjectId) > 0 {
		info["manager_project_id"] = provider.ProjectId
		tc, err := db.TenantCacheManager.FetchTenantById(appctx.Background, provider.ProjectId)
		if err == nil {
			info["manager_project"] = tc.GetName()
		}
	}

	account := provider.GetCloudaccount()
	info["account"] = account.GetName()
	info["account_id"] = account.GetId()

	return jsonutils.Marshal(info).(*jsonutils.JSONDict)
}

func (self *SManagedResourceBase) GetProviderFactory() (cloudprovider.ICloudProviderFactory, error) {
	provider := self.GetCloudprovider()
	if provider == nil {
		if len(self.ManagerId) > 0 {
			return nil, cloudprovider.ErrInvalidProvider
		}
		return nil, fmt.Errorf("Resource is self managed")
	}
	return provider.GetProviderFactory()
}

func (self *SManagedResourceBase) GetDriver() (cloudprovider.ICloudProvider, error) {
	provider := self.GetCloudprovider()
	if provider == nil {
		if len(self.ManagerId) > 0 {
			return nil, cloudprovider.ErrInvalidProvider
		}
		return nil, fmt.Errorf("Resource is self managed")
	}
	return provider.GetProvider()
}

func (self *SManagedResourceBase) GetProviderName() string {
	driver := self.GetCloudprovider()
	if driver != nil {
		return driver.Provider
	}
	return ""
}

func (self *SManagedResourceBase) IsManaged() bool {
	return len(self.ManagerId) > 0
}

func managedResourceFilterByDomain(q *sqlchemy.SQuery, query jsonutils.JSONObject, filterField string, subqFunc func() *sqlchemy.SQuery) (*sqlchemy.SQuery, error) {
	domainStr, key := jsonutils.GetAnyString2(query, []string{"domain_id", "project_domain", "project_domain_id"})
	if len(domainStr) > 0 {
		query.(*jsonutils.JSONDict).Remove(key)
		domain, err := db.TenantCacheManager.FetchDomainByIdOrName(context.Background(), domainStr)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2("domains", domainStr)
			}
			return nil, httperrors.NewGeneralError(err)
		}
		accounts := CloudaccountManager.Query().SubQuery()
		providers := CloudproviderManager.Query().SubQuery()
		subq := providers.Query(providers.Field("id"))
		subq = subq.Join(accounts, sqlchemy.Equals(providers.Field("cloudaccount_id"), accounts.Field("id")))
		subq = subq.Filter(sqlchemy.Equals(accounts.Field("domain_id"), domain.GetId()))
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.In(q.Field("manager_id"), subq.SubQuery()))
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.In(sq.Field("manager_id"), subq.SubQuery()))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
		}
	}
	return q, nil
}

func managedResourceFilterByAccount(q *sqlchemy.SQuery, query jsonutils.JSONObject, filterField string, subqFunc func() *sqlchemy.SQuery) (*sqlchemy.SQuery, error) {
	queryDict := query.(*jsonutils.JSONDict)

	managerStr, key := jsonutils.GetAnyString2(query, []string{"manager", "cloudprovider", "cloudprovider_id", "manager_id"})
	if len(managerStr) > 0 {
		queryDict.Remove("manager")
		queryDict.Remove(key)
		provider, err := CloudproviderManager.FetchByIdOrName(nil, managerStr)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2(CloudproviderManager.Keyword(), managerStr)
			}
			return nil, httperrors.NewGeneralError(err)
		}
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.Equals(q.Field("manager_id"), provider.GetId()))
			queryDict.Remove("manager_id")
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.Equals(sq.Field("manager_id"), provider.GetId()))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
			queryDict.Remove(filterField)
		}
	}

	accountStr, key := jsonutils.GetAnyString2(query, []string{"account", "account_id", "cloudaccount", "cloudaccount_id"})
	if len(accountStr) > 0 {
		queryDict.Remove("account")
		queryDict.Remove(key)
		account, err := CloudaccountManager.FetchByIdOrName(nil, accountStr)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2(CloudaccountManager.Keyword(), accountStr)
			}
			return nil, httperrors.NewGeneralError(err)
		}
		subq := CloudproviderManager.Query("id").Equals("cloudaccount_id", account.GetId()).SubQuery()
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.In(q.Field("manager_id"), subq))
			queryDict.Remove("manager_id")
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.In(sq.Field("manager_id"), subq))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
			queryDict.Remove(filterField)
		}
	}

	providerStr := jsonutils.GetAnyString(query, []string{"provider"})
	if len(providerStr) > 0 {
		queryDict.Remove("provider")
		if providerStr == api.CLOUD_PROVIDER_ONECLOUD {
			if len(filterField) == 0 {
				q = q.Filter(sqlchemy.IsNullOrEmpty(q.Field("manager_id")))
			} else {
				sq := subqFunc()
				sq = sq.Filter(sqlchemy.IsNullOrEmpty(sq.Field("manager_id")))
				q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
				queryDict.Remove(filterField)
			}
		} else {
			subq := CloudproviderManager.Query("id").Equals("provider", providerStr).SubQuery()
			if len(filterField) == 0 {
				q = q.Filter(sqlchemy.In(q.Field("manager_id"), subq))
				queryDict.Remove("manager_id")
			} else {
				sq := subqFunc()
				sq = sq.Filter(sqlchemy.In(sq.Field("manager_id"), subq))
				q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
				queryDict.Remove(filterField)
			}
		}
	}

	brandStr := jsonutils.GetAnyString(query, []string{"brand"})
	if len(brandStr) > 0 {
		queryDict.Remove("brand")

		if brandStr == api.CLOUD_PROVIDER_ONECLOUD {
			if len(filterField) == 0 {
				q = q.Filter(sqlchemy.IsNullOrEmpty(q.Field("manager_id")))
			} else {
				sq := subqFunc()
				sq = sq.Filter(sqlchemy.IsNullOrEmpty(sq.Field("manager_id")))
				q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
				queryDict.Remove(filterField)
			}
		} else {
			account := CloudaccountManager.Query().SubQuery()
			providers := CloudproviderManager.Query().SubQuery()
			subq := providers.Query(providers.Field("id"))
			subq = subq.Join(account, sqlchemy.Equals(
				account.Field("id"), providers.Field("cloudaccount_id"),
			))
			subq = subq.Filter(sqlchemy.Equals(account.Field("brand"), brandStr))

			if len(filterField) == 0 {
				q = q.Filter(sqlchemy.In(q.Field("manager_id"), subq.SubQuery()))
				queryDict.Remove("manager_id")
			} else {
				sq := subqFunc()
				sq = sq.Filter(sqlchemy.In(sq.Field("manager_id"), subq.SubQuery()))
				q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
				queryDict.Remove(filterField)
			}
		}
	}

	return q, nil
}

func managedResourceFilterByZone(q *sqlchemy.SQuery, query jsonutils.JSONObject, filterField string, subqFunc func() *sqlchemy.SQuery) (*sqlchemy.SQuery, error) {
	zoneStr, key := jsonutils.GetAnyString2(query, []string{"zone", "zone_id"})
	if len(zoneStr) > 0 {
		query.(*jsonutils.JSONDict).Remove(key)
		zoneObj, err := ZoneManager.FetchByIdOrName(nil, zoneStr)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2(ZoneManager.Keyword(), zoneStr)
			} else {
				return nil, httperrors.NewGeneralError(err)
			}
		}
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.Equals(q.Field("zone_id"), zoneObj.GetId()))
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.Equals(sq.Field("zone_id"), zoneObj.GetId()))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
		}
	}
	return q, nil
}

func managedResourceFilterByRegion(q *sqlchemy.SQuery, query jsonutils.JSONObject, filterField string, subqFunc func() *sqlchemy.SQuery) (*sqlchemy.SQuery, error) {
	regionStr, key := jsonutils.GetAnyString2(query, []string{"region", "region_id", "cloudregion", "cloudregion_id"})
	if len(regionStr) > 0 {
		query.(*jsonutils.JSONDict).Remove(key)
		regionObj, err := CloudregionManager.FetchByIdOrName(nil, regionStr)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2(CloudregionManager.Keyword(), regionStr)
			} else {
				return nil, httperrors.NewGeneralError(err)
			}
		}
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.In(q.Field("cloudregion_id"), regionObj.GetId()))
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.In(sq.Field("cloudregion_id"), regionObj.GetId()))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
		}
	}
	return q, nil
}

func managedResourceFilterByCloudType(q *sqlchemy.SQuery, query jsonutils.JSONObject, filterField string, subqFunc func() *sqlchemy.SQuery) *sqlchemy.SQuery {
	cloudEnvStr, _ := query.GetString("cloud_env")

	if cloudEnvStr == api.CLOUD_ENV_PUBLIC_CLOUD || jsonutils.QueryBoolean(query, "public_cloud", false) || jsonutils.QueryBoolean(query, "is_public", false) {
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.In(q.Field("manager_id"), CloudproviderManager.GetPublicProviderIdsQuery()))
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.In(sq.Field("manager_id"), CloudproviderManager.GetPublicProviderIdsQuery()))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
		}
	}

	if cloudEnvStr == api.CLOUD_ENV_PRIVATE_CLOUD || jsonutils.QueryBoolean(query, "private_cloud", false) || jsonutils.QueryBoolean(query, "is_private", false) {
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.In(q.Field("manager_id"), CloudproviderManager.GetPrivateProviderIdsQuery()))
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.In(sq.Field("manager_id"), CloudproviderManager.GetPrivateProviderIdsQuery()))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
		}
	}

	if cloudEnvStr == api.CLOUD_ENV_ON_PREMISE || jsonutils.QueryBoolean(query, "is_on_premise", false) {
		if len(filterField) == 0 {
			q = q.Filter(
				sqlchemy.OR(
					sqlchemy.In(q.Field("manager_id"), CloudproviderManager.GetOnPremiseProviderIdsQuery()),
					sqlchemy.IsNullOrEmpty(q.Field("manager_id")),
				),
			)
		} else {
			sq := subqFunc()
			sq = sq.Filter(
				sqlchemy.OR(
					sqlchemy.In(sq.Field("manager_id"), CloudproviderManager.GetOnPremiseProviderIdsQuery()),
					sqlchemy.IsNullOrEmpty(sq.Field("manager_id")),
				),
			)
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
		}
	}

	if jsonutils.QueryBoolean(query, "is_managed", false) {
		if len(filterField) == 0 {
			q = q.Filter(sqlchemy.IsNotEmpty(q.Field("manager_id")))
		} else {
			sq := subqFunc()
			sq = sq.Filter(sqlchemy.IsNotEmpty(sq.Field("manager_id")))
			q = q.Filter(sqlchemy.In(q.Field(filterField), sq.SubQuery()))
		}
	}

	return q
}

type SCloudProviderInfo struct {
	Provider         string `json:",omitempty"`
	Brand            string `json:",omitempty"`
	Account          string `json:",omitempty"`
	AccountId        string `json:",omitempty"`
	Manager          string `json:",omitempty"`
	ManagerId        string `json:",omitempty"`
	ManagerProject   string `json:",omitempty"`
	ManagerProjectId string `json:",omitempty"`
	ManagerDomain    string `json:",omitempty"`
	ManagerDomainId  string `json:",omitempty"`
	Region           string `json:",omitempty"`
	RegionId         string `json:",omitempty"`
	RegionExtId      string `json:",omitempty"`
	Zone             string `json:",omitempty"`
	ZoneId           string `json:",omitempty"`
	ZoneExtId        string `json:",omitempty"`
	CloudEnv         string `json:",omitempty"`
}

var (
	providerInfoFields = []string{
		"provider",
		"brand",
		"account",
		"account_id",
		"manager",
		"manager_id",
		"manager_project",
		"manager_project_id",
		"region",
		"region_id",
		"region_ext_id",
		"zone",
		"zone_id",
		"zone_ext_id",
		"cloud_env",
	}
)

func fetchExternalId(extId string) string {
	pos := strings.LastIndexByte(extId, '/')
	if pos > 0 {
		return extId[pos+1:]
	} else {
		return extId
	}
}

func MakeCloudProviderInfo(region *SCloudregion, zone *SZone, provider *SCloudprovider) SCloudProviderInfo {
	info := SCloudProviderInfo{}

	if zone != nil {
		info.Zone = zone.GetName()
		info.ZoneId = zone.GetId()
	}

	if region != nil {
		info.Region = region.GetName()
		info.RegionId = region.GetId()
	}

	if provider != nil {
		info.Manager = provider.GetName()
		info.ManagerId = provider.GetId()

		if len(provider.ProjectId) > 0 {
			info.ManagerProjectId = provider.ProjectId
			tc, err := db.TenantCacheManager.FetchTenantById(appctx.Background, provider.ProjectId)
			if err == nil {
				info.ManagerProject = tc.GetName()
				info.ManagerDomain = tc.Domain
				info.ManagerDomainId = tc.DomainId
			}
		}

		account := provider.GetCloudaccount()
		info.Account = account.GetName()
		info.AccountId = account.GetId()

		info.Provider = provider.Provider
		info.Brand = account.Brand
		info.CloudEnv = account.getCloudEnv()

		if region != nil {
			info.RegionExtId = fetchExternalId(region.ExternalId)
			if zone != nil {
				info.ZoneExtId = fetchExternalId(zone.ExternalId)
			}
		}
	} else {
		info.CloudEnv = api.CLOUD_ENV_ON_PREMISE
		info.Provider = api.CLOUD_PROVIDER_ONECLOUD
		info.Brand = api.CLOUD_PROVIDER_ONECLOUD
	}

	return info
}

func fetchByManagerId(manager db.IModelManager, providerId string, receiver interface{}) error {
	q := manager.Query().Equals("manager_id", providerId)
	return db.FetchModelObjects(manager, q, receiver)
}
