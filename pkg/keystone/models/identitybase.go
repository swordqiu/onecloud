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

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/tristate"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/identity"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type IIdentityModelManager interface {
	db.IStandaloneModelManager

	GetIIdentityModelManager() IIdentityModelManager
}

type IIdentityModel interface {
	db.IStandaloneModel

	GetIIdentityModelManager() IIdentityModelManager

	GetIIdentityModel() IIdentityModel
}

type SIdentityBaseResourceManager struct {
	db.SStandaloneResourceBaseManager
	db.SDomainizedResourceBaseManager
}

func NewIdentityBaseResourceManager(dt interface{}, tableName string, keyword string, keywordPlural string) SIdentityBaseResourceManager {
	return SIdentityBaseResourceManager{
		SStandaloneResourceBaseManager: db.NewStandaloneResourceBaseManager(dt, tableName, keyword, keywordPlural),
	}
}

type SIdentityBaseResource struct {
	db.SStandaloneResourceBase
	db.SDomainizedResourceBase

	Extra *jsonutils.JSONDict `nullable:"true"`
	// DomainId string `width:"64" charset:"ascii" default:"default" nullable:"false" index:"true" list:"user"`
}

type SEnabledIdentityBaseResourceManager struct {
	SIdentityBaseResourceManager
}

func NewEnabledIdentityBaseResourceManager(dt interface{}, tableName string, keyword string, keywordPlural string) SEnabledIdentityBaseResourceManager {
	return SEnabledIdentityBaseResourceManager{
		SIdentityBaseResourceManager: NewIdentityBaseResourceManager(dt, tableName, keyword, keywordPlural),
	}
}

type SEnabledIdentityBaseResource struct {
	SIdentityBaseResource

	Enabled tristate.TriState `nullable:"false" default:"true" list:"admin" update:"admin" create:"admin_optional"`
}

func (model *SIdentityBaseResource) GetIIdentityModelManager() IIdentityModelManager {
	return model.GetModelManager().(IIdentityModelManager)
}

func (model *SIdentityBaseResource) GetIIdentityModel() IIdentityModel {
	return model.GetVirtualObject().(IIdentityModel)
}

func (model *SIdentityBaseResource) IsOwner(userCred mcclient.TokenCredential) bool {
	return userCred.GetProjectDomainId() == model.DomainId
}

func (model *SIdentityBaseResource) GetDomain() *SDomain {
	if len(model.DomainId) > 0 && model.DomainId != api.KeystoneDomainRoot {
		domain, err := DomainManager.FetchDomainById(model.DomainId)
		if err != nil {
			log.Errorf("GetDomain fail %s", err)
		}
		return domain
	}
	return nil
}

func (manager *SIdentityBaseResourceManager) GetIIdentityModelManager() IIdentityModelManager {
	return manager.GetVirtualObject().(IIdentityModelManager)
}

func (manager *SIdentityBaseResourceManager) FetchByName(userCred mcclient.IIdentityProvider, idStr string) (db.IModel, error) {
	return db.FetchByName(manager, userCred, idStr)
}

func (manager *SIdentityBaseResourceManager) FetchByIdOrName(userCred mcclient.IIdentityProvider, idStr string) (db.IModel, error) {
	return db.FetchByIdOrName(manager, userCred, idStr)
}

func (manager *SIdentityBaseResourceManager) ListItemFilter(ctx context.Context, q *sqlchemy.SQuery, userCred mcclient.TokenCredential, query jsonutils.JSONObject) (*sqlchemy.SQuery, error) {
	q, err := manager.SStandaloneResourceBaseManager.ListItemFilter(ctx, q, userCred, query)
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (manager *SIdentityBaseResourceManager) FetchOwnerId(ctx context.Context, data jsonutils.JSONObject) (mcclient.IIdentityProvider, error) {
	domainId := jsonutils.GetAnyString(data, []string{"domain", "domain_id", "project_domain", "project_domain_id"})
	if len(domainId) > 0 {
		domain, err := DomainManager.FetchDomainByIdOrName(domainId)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2(DomainManager.Keyword(), domainId)
			}
			return nil, httperrors.NewGeneralError(err)
		}
		owner := db.SOwnerId{DomainId: domain.Id, Domain: domain.Name}
		return &owner, nil
	}
	return nil, nil
}

func (manager *SIdentityBaseResourceManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	domain, _ := DomainManager.FetchDomainById(ownerId.GetProjectDomainId())
	if domain.Enabled.IsFalse() {
		return nil, httperrors.NewInvalidStatusError("domain is disabled")
	}
	return manager.SStandaloneResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, data)
}

func (manager *SIdentityBaseResourceManager) NamespaceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeSystem
}

func (manager *SIdentityBaseResourceManager) FetchCustomizeColumns(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, objs []db.IModel, fields stringutils2.SSortedStrings) []*jsonutils.JSONDict {
	rows := manager.SStandaloneResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields)
	if len(fields) == 0 || fields.Contains("domain") {
		domainIds := stringutils2.SSortedStrings{}
		for i := range objs {
			idStr := objs[i].GetOwnerId().GetProjectDomainId()
			if idStr != api.KeystoneDomainRoot {
				domainIds = stringutils2.Append(domainIds, idStr)
			}
		}
		log.Debugf("expand domain ... %s", domainIds)
		domains := fetchDomain(domainIds)
		if domains != nil {
			for i := range rows {
				idStr := objs[i].GetOwnerId().GetProjectDomainId()
				if idStr != api.KeystoneDomainRoot {
					if domain, ok := domains[idStr]; ok {
						if len(fields) == 0 || fields.Contains("domain") {
							rows[i].Add(jsonutils.NewString(domain.Name), "domain")
						}
					}
				}
			}
		}
	}
	return rows
}

func fetchDomain(domainIds []string) map[string]SDomain {
	q := DomainManager.Query().In("id", domainIds)
	domains := make([]SDomain, 0)
	err := db.FetchModelObjects(DomainManager, q, &domains)
	if err != nil {
		return nil
	}
	ret := make(map[string]SDomain)
	for i := range domains {
		ret[domains[i].Id] = domains[i]
	}
	return ret
}

func (model *SIdentityBaseResource) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	model.DomainId = ownerId.GetProjectDomainId()
	return model.SStandaloneResourceBase.CustomizeCreate(ctx, userCred, ownerId, query, data)
}

func (self *SIdentityBaseResource) ValidateDeleteCondition(ctx context.Context) error {
	// domain := self.GetDomain()
	// if self.GetIIdentityModelManager().IsDomainReadonly(domain) {
	// 	return httperrors.NewForbiddenError("readonly domain")
	// }
	return self.SStandaloneResourceBase.ValidateDeleteCondition(ctx)
}

func (self *SIdentityBaseResource) ValidateUpdateData(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	// if data.Contains("name") {
	//	domain := self.GetDomain()
	//	if self.GetIIdentityModelManager().IsDomainReadonly(domain) {
	//		return nil, httperrors.NewForbiddenError("cannot update name in readonly domain")
	// 	}
	// }
	return self.SStandaloneResourceBase.ValidateUpdateData(ctx, userCred, query, data)
}

func (self *SEnabledIdentityBaseResource) ValidateDeleteCondition(ctx context.Context) error {
	if self.Enabled.IsTrue() {
		return httperrors.NewResourceBusyError("resource is enabled")
	}
	return self.SIdentityBaseResource.ValidateDeleteCondition(ctx)
}
