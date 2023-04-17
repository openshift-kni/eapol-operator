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

package k8s

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
	"github.com/openshift-kni/eapol-operator/pkg/configgen"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetClient() (client.Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	scheme := runtime.NewScheme()
	if err := eapolv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return client.New(config, client.Options{
		Scheme: scheme,
	})
}

func GetAuthNamespacedName() (*types.NamespacedName, error) {
	// Open labels File
	labelsPath := filepath.Join(configgen.AuthenticatorMountPath, "labels")
	file, err := os.Open(labelsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var authNs, authName string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		label := strings.Split(string(line), "\n")
		for _, l := range label {
			parts := strings.Split(string(l), "=")
			if len(parts) != 2 {
				continue
			}
			parts[1] = strings.Replace(string(parts[1]), "\\n", "", -1)
			parts[1] = strings.Replace(string(parts[1]), "\\", "", -1)
			parts[1] = strings.Replace(string(parts[1]), " ", "", -1)
			parts[1] = string(parts[1][1 : len(parts[1])-1])

			if parts[0] == configgen.AuthNamespace {
				authNs = parts[1]
			} else if parts[0] == configgen.AuthName {
				authName = parts[1]
			}
		}
	}
	if authNs == "" || authName == "" {
		return nil, errors.New("no authentication object found from labels")
	}
	return &types.NamespacedName{Namespace: authNs, Name: authName}, nil
}
