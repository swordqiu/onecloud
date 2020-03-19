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
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/reflectutils"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SSecurityGroupResourceBase struct {
	// 本地安全组ID
	SecgroupId string `width:"36" charset:"ascii" nullable:"false" create:"required"  index:"true" list:"user"`
}

type SSecurityGroupResourceBaseManager struct{}

func (self *SSecurityGroupResourceBase) GetSecGroup() *SSecurityGroup {
	secgrp, err := SecurityGroupManager.FetchById(self.SecgroupId)
	if err != nil {
		log.Errorf("failed to find secgroup %s error: %v", self.SecgroupId, err)
		return nil
	}
	return secgrp.(*SSecurityGroup)
}

func (self *SSecurityGroupResourceBaseManager) GetExtraDetails(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	isList bool,
) api.SecurityGroupResourceInfo {
	return api.SecurityGroupResourceInfo{}
}

func (manager *SSecurityGroupResourceBaseManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.SecurityGroupResourceInfo {
	rows := make([]api.SecurityGroupResourceInfo, len(objs))
	secgrpIds := make([]string, len(objs))
	for i := range objs {
		var base *SSecurityGroupResourceBase
		err := reflectutils.FindAnonymouStructPointer(objs[i], &base)
		if err != nil {
			log.Errorf("Cannot find SSecurityGroupResourceBase in object %s", objs[i])
			continue
		}
		secgrpIds[i] = base.SecgroupId
	}
	secgrpNames, err := db.FetchIdNameMap2(SecurityGroupManager, secgrpIds)
	if err != nil {
		log.Errorf("FetchIdNameMap2 fail %s", err)
		return rows
	}
	for i := range rows {
		if name, ok := secgrpNames[secgrpIds[i]]; ok {
			rows[i].Secgroup = name
		}
	}
	return rows
}

func (manager *SSecurityGroupResourceBaseManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.SecgroupFilterListInput,
) (*sqlchemy.SQuery, error) {
	if len(query.Secgroup) > 0 {
		secgrpObj, err := SecurityGroupManager.FetchByIdOrName(userCred, query.Secgroup)
		if err != nil {
			if errors.Cause(err) == sql.ErrNoRows {
				return nil, httperrors.NewResourceNotFoundError2(SecurityGroupManager.Keyword(), query.Secgroup)
			} else {
				return nil, errors.Wrap(err, "SecurityGroupManager.FetchByIdOrName")
			}
		}
		q = q.Equals("secgroup_id", secgrpObj.GetId())
	}
	return q, nil
}

func (manager *SSecurityGroupResourceBaseManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.SecgroupFilterListInput,
) (*sqlchemy.SQuery, error) {
	q, orders, fields := manager.GetOrderBySubQuery(q, userCred, query)
	if len(orders) > 0 {
		q = db.OrderByFields(q, orders, fields)
	}
	return q, nil
}

func (manager *SSecurityGroupResourceBaseManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	if field == "secgroup" {
		secgrpQuery := SecurityGroupManager.Query("name", "id").Distinct().SubQuery()
		q.AppendField(secgrpQuery.Field("name", field))
		q = q.Join(secgrpQuery, sqlchemy.Equals(q.Field("secgroup_id"), secgrpQuery.Field("id")))
		q.GroupBy(secgrpQuery.Field("name"))
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

func (manager *SSecurityGroupResourceBaseManager) GetOrderBySubQuery(
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.SecgroupFilterListInput,
) (*sqlchemy.SQuery, []string, []sqlchemy.IQueryField) {
	secgrpQ := SecurityGroupManager.Query("id", "name")
	var orders []string
	var fields []sqlchemy.IQueryField
	if db.NeedOrderQuery(manager.GetOrderByFields(query)) {
		subq := secgrpQ.SubQuery()
		q = q.LeftJoin(subq, sqlchemy.Equals(q.Field("secgroup_id"), subq.Field("id")))
		orders = append(orders, query.OrderBySecgroup)
		fields = append(fields, subq.Field("name"))
	}
	return q, orders, fields
}

func (manager *SSecurityGroupResourceBaseManager) GetOrderByFields(query api.SecgroupFilterListInput) []string {
	return []string{query.OrderBySecgroup}
}
