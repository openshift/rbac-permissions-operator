// Copyright 2019 RedHat
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

package permissioncheck

import (
	"fmt"

	auth "k8s.io/api/authorization/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// PermissionCheck - wrapper around a Kubernetes client.
type PermissionCheck struct {
	clientset kubernetes.Interface
}

// NewPermissionCheck - set up for Permission Checking with
// SelfSubjectAccessReviews with the provided *rest.Config.
func NewPermissionCheck(cfg *rest.Config) *PermissionCheck {
	ret := &PermissionCheck{}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err.Error())
	}
	ret.clientset = cs
	return ret
}

// CheckPermissions will iterate through all of the desired perms and check
// access. The method returns true if, and only if all desired permissions are
// allowable.
// Return values are:
// bool - if all the permissions represented in +perms+ can be done
// error - if any error occurred while performing any of the permission checks.
func (p *PermissionCheck) CheckPermissions(perms []auth.ResourceAttributes) (bool, error) {
	allowed := true

	for _, perm := range perms {
		pCopy := perm.DeepCopy()
		ssar := &auth.SelfSubjectAccessReview{
			Spec: auth.SelfSubjectAccessReviewSpec{
				ResourceAttributes: pCopy,
			},
		}
		ok, err := p.checkPermission(ssar)
		if err != nil {
			return false, err
		}
		allowed = allowed && ok
		if !allowed {
			// no sense checking further if we already sense failure!
			return false, fmt.Errorf("Permission denied to %s:%s", pCopy, ssar.Status.Reason)
		}
		ssar = nil
	}
	return allowed, nil
}

// checkPermission takes the SSAR and returns bool if the access is allowed.
func (p *PermissionCheck) checkPermission(ssar *auth.SelfSubjectAccessReview) (bool, error) {
	ssar, err := p.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ssar)
	if err != nil {
		return false, err
	}
	return ssar.Status.Allowed, nil
}
