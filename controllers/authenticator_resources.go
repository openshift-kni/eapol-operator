/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Note that most of logic for this module is copied from https://github.com/openshift/cluster-nfd-operator/blob/master/controllers/nodefeaturediscovery_resources.go
// (as of latest commit 46ef970beb2227ad8363df5099c63d2cd2150714).

package controllers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

// assetsFromFile is the content of an asset file as raw data
type assetsFromFile []byte

// resources holds objects owned by Authenticator
type resources struct {
	serviceAccount *corev1.ServiceAccount
	role           *rbacv1.Role
	roleBinding    *rbacv1.RoleBinding
}

// filePathWalkDir finds all non-directory files under the given path recursively,
// i.e. including its subdirectories
func filePathWalkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// getAssetsFrom recursively reads all manifest files under a given path
func getAssetsFrom(path string) []assetsFromFile {
	// All assets (manifests) as raw data
	manifests := []assetsFromFile{}
	assets := path

	// For the given path, find a list of all the files
	files, err := filePathWalkDir(assets)
	if err != nil {
		panic(err)
	}

	// For each file in the 'files' list, read the file
	// and store its contents in 'manifests'
	for _, file := range files {
		buffer, err := os.ReadFile(file)
		if err != nil {
			panic(err)
		}

		manifests = append(manifests, buffer)
	}
	return manifests
}

func retrieveResources(path string) (*resources, error) {
	// Information about the manifest
	res := &resources{}

	// Get the list of manifests from the given path
	manifests := getAssetsFrom(path)

	// s and reg are used later on to parse the manifest YAML
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, runtime.NewScheme(),
		runtime.NewScheme())
	reg, _ := regexp.Compile(`\b(\w*kind:\w*)\B.*\b`)

	// Append the appropriate control function depending on the kind
	for _, m := range manifests {
		kind := reg.FindString(string(m))
		slce := strings.Split(kind, ":")
		kind = strings.TrimSpace(slce[1])

		switch kind {
		case "ServiceAccount":
			sa := &corev1.ServiceAccount{}
			_, _, err := s.Decode(m, nil, sa)
			panicIfError(err)
			res.serviceAccount = sa
		case "Role":
			r := &rbacv1.Role{}
			_, _, err := s.Decode(m, nil, r)
			panicIfError(err)
			res.role = r
		case "RoleBinding":
			rb := &rbacv1.RoleBinding{}
			_, _, err := s.Decode(m, nil, rb)
			panicIfError(err)
			if len(rb.Subjects) != 1 {
				return nil, fmt.Errorf("invalid subject length on authenticator rbac binding")
			}
			res.roleBinding = rb
		default:
			return nil, fmt.Errorf("unknown resource: kind %s", kind)
		}
	}
	if res.serviceAccount == nil {
		return nil, errors.New("authenticator service account object not found")
	}
	if res.role == nil {
		return nil, errors.New("authenticator role object not found")
	}
	if res.roleBinding == nil {
		return nil, errors.New("authenticator role binding object not found")
	}

	return res, nil
}

// panicIfError panics in case of an error
func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
