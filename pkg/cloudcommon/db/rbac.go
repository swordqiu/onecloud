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

package db

import (
	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/onecloud/pkg/cloudcommon/policy"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
)

func isObjectRbacAllowed(model IModel, userCred mcclient.TokenCredential, action string, extra ...string) bool {
	manager := model.GetModelManager()
	objOwnerId := model.GetOwnerId()

	var ownerId mcclient.IIdentityProvider
	if userCred != nil {
		ownerId = userCred
	}

	var requireScope rbacutils.TRbacScope
	resScope := manager.ResourceScope()
	switch resScope {
	case rbacutils.ScopeSystem:
		requireScope = rbacutils.ScopeSystem
	case rbacutils.ScopeDomain:
		// objOwnerId should not be nil
		if ownerId != nil && (ownerId.GetProjectDomainId() == objOwnerId.GetProjectDomainId() || (model.IsSharable(ownerId) && action == policy.PolicyActionGet)) {
			requireScope = rbacutils.ScopeDomain
		} else {
			requireScope = rbacutils.ScopeSystem
		}
	case rbacutils.ScopeUser:
		if ownerId != nil && (ownerId.GetUserId() == objOwnerId.GetUserId() || (model.IsSharable(ownerId) && action == policy.PolicyActionGet)) {
			requireScope = rbacutils.ScopeUser
		} else {
			requireScope = rbacutils.ScopeSystem
		}
	default:
		// objOwnerId should not be nil
		if ownerId != nil && (ownerId.GetProjectId() == objOwnerId.GetProjectId() || (model.IsSharable(ownerId) && action == policy.PolicyActionGet)) {
			requireScope = rbacutils.ScopeProject
		} else if ownerId != nil && ownerId.GetProjectDomainId() == objOwnerId.GetProjectDomainId() {
			requireScope = rbacutils.ScopeDomain
		} else {
			requireScope = rbacutils.ScopeSystem
		}
	}

	scope := policy.PolicyManager.AllowScope(userCred, consts.GetServiceType(), manager.KeywordPlural(), action, extra...)

	if !requireScope.HigherThan(scope) {
		return true
	}

	return false
}

func isJointObjectRbacAllowed(item IJointModel, userCred mcclient.TokenCredential, action string, extra ...string) bool {
	return isObjectRbacAllowed(item.Master(), userCred, action, extra...) && isObjectRbacAllowed(item.Slave(), userCred, action, extra...)
}

func isClassRbacAllowed(manager IModelManager, userCred mcclient.TokenCredential, objOwnerId mcclient.IIdentityProvider, action string, extra ...string) bool {
	var ownerId mcclient.IIdentityProvider
	if userCred != nil {
		ownerId = userCred
	}

	var requireScope rbacutils.TRbacScope
	resScope := manager.ResourceScope()
	switch resScope {
	case rbacutils.ScopeSystem:
		requireScope = rbacutils.ScopeSystem
	case rbacutils.ScopeDomain:
		// objOwnerId should not be nil
		if ownerId != nil && ownerId.GetProjectDomainId() == objOwnerId.GetProjectDomainId() {
			requireScope = rbacutils.ScopeDomain
		} else {
			requireScope = rbacutils.ScopeSystem
		}
	case rbacutils.ScopeUser:
		if ownerId != nil && ownerId.GetUserId() == objOwnerId.GetUserId() {
			requireScope = rbacutils.ScopeUser
		} else {
			requireScope = rbacutils.ScopeSystem
		}
	default:
		// objOwnerId should not be nil
		if ownerId != nil && ownerId.GetProjectId() == objOwnerId.GetProjectId() {
			requireScope = rbacutils.ScopeProject
		} else if ownerId != nil && ownerId.GetProjectDomainId() == objOwnerId.GetProjectDomainId() {
			requireScope = rbacutils.ScopeDomain
		} else {
			requireScope = rbacutils.ScopeSystem
		}
	}

	allowScope := policy.PolicyManager.AllowScope(userCred, consts.GetServiceType(), manager.KeywordPlural(), action, extra...)

	if !requireScope.HigherThan(allowScope) {
		return true
	}

	return false
}

type IResource interface {
	KeywordPlural() string
}

func IsAllowList(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, manager IResource) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), manager.KeywordPlural(), policy.PolicyActionList)
}

func IsAdminAllowList(userCred mcclient.TokenCredential, manager IResource) bool {
	return IsAllowList(rbacutils.ScopeSystem, userCred, manager)
}

func IsAllowCreate(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, manager IResource) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), manager.KeywordPlural(), policy.PolicyActionCreate)
}

func IsAdminAllowCreate(userCred mcclient.TokenCredential, manager IResource) bool {
	return IsAllowCreate(rbacutils.ScopeSystem, userCred, manager)
}

func IsAllowClassPerform(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, manager IResource, action string) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), manager.KeywordPlural(), policy.PolicyActionPerform, action)
}

func IsAdminAllowClassPerform(userCred mcclient.TokenCredential, manager IResource, action string) bool {
	return IsAllowClassPerform(rbacutils.ScopeSystem, userCred, manager, action)
}

func IsAllowGet(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, obj IResource) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), obj.KeywordPlural(), policy.PolicyActionGet)
}

func IsAdminAllowGet(userCred mcclient.TokenCredential, obj IResource) bool {
	return IsAllowGet(rbacutils.ScopeSystem, userCred, obj)
}

func IsAllowGetSpec(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, obj IResource, spec string) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), obj.KeywordPlural(), policy.PolicyActionGet, spec)
}

func IsAdminAllowGetSpec(userCred mcclient.TokenCredential, obj IResource, spec string) bool {
	return IsAllowGetSpec(rbacutils.ScopeSystem, userCred, obj, spec)
}

func IsAllowPerform(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, obj IResource, action string) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), obj.KeywordPlural(), policy.PolicyActionPerform, action)
}

func IsAdminAllowPerform(userCred mcclient.TokenCredential, obj IResource, action string) bool {
	return IsAllowPerform(rbacutils.ScopeSystem, userCred, obj, action)
}

func IsAllowUpdate(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, obj IResource) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), obj.KeywordPlural(), policy.PolicyActionUpdate)
}

func IsAdminAllowUpdate(userCred mcclient.TokenCredential, obj IResource) bool {
	return IsAllowUpdate(rbacutils.ScopeSystem, userCred, obj)
}

func IsAllowUpdateSpec(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, obj IResource, spec string) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), obj.KeywordPlural(), policy.PolicyActionUpdate, spec)
}

func IsAdminAllowUpdateSpec(userCred mcclient.TokenCredential, obj IResource, spec string) bool {
	return IsAllowUpdateSpec(rbacutils.ScopeSystem, userCred, obj, spec)
}

func IsAllowDelete(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, obj IResource) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), obj.KeywordPlural(), policy.PolicyActionDelete)
}

func IsAdminAllowDelete(userCred mcclient.TokenCredential, obj IResource) bool {
	return IsAllowDelete(rbacutils.ScopeSystem, userCred, obj)
}

func IsAllowDeleteSpec(scope rbacutils.TRbacScope, userCred mcclient.TokenCredential, obj IResource, spec string) bool {
	if userCred == nil {
		return false
	}
	return userCred.IsAllow(scope, consts.GetServiceType(), obj.KeywordPlural(), policy.PolicyActionDelete, spec)
}

func IsAdminAllowDeleteSpec(userCred mcclient.TokenCredential, obj IResource, spec string) bool {
	return IsAllowDeleteSpec(rbacutils.ScopeSystem, userCred, obj, spec)
}
