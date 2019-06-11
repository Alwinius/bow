/*
Copyright The Helm Authors.

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

package helm

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/downloader"
	"k8s.io/helm/pkg/getter"
	"k8s.io/helm/pkg/kube"
	"k8s.io/helm/pkg/repo"
)

type valueFiles []string

func (v *valueFiles) String() string {
	return fmt.Sprint(*v)
}

func (v *valueFiles) Type() string {
	return "valueFiles"
}

func (v *valueFiles) Set(value string) error {
	for _, filePath := range strings.Split(value, ",") {
		*v = append(*v, filePath)
	}
	return nil
}

// Merges source and destination map, preferring values from the source map
func mergeValues(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	for k, v := range src {
		// If the key doesn't exist already, then just set the key to that value
		if _, exists := dest[k]; !exists {
			dest[k] = v
			continue
		}
		nextMap, ok := v.(map[string]interface{})
		// If it isn't another map, overwrite the value
		if !ok {
			dest[k] = v
			continue
		}
		// Edge case: If the key exists in the destination, but isn't a map
		destMap, isMap := dest[k].(map[string]interface{})
		// If the source map has a map for this key, prefer it
		if !isMap {
			dest[k] = v
			continue
		}
		// If we got to this point, it is a map in both, so merge them
		dest[k] = mergeValues(destMap, nextMap)
	}
	return dest
}

// vals merges values from files specified via -f/--values and
// directly via --set or --set-string or --set-file, marshaling them to YAML
func vals(valueFiles valueFiles, CertFile, KeyFile, CAFile string) ([]byte, error) {
	base := map[string]interface{}{}

	// User specified a values files via -f/--values
	for _, filePath := range valueFiles {
		currentMap := map[string]interface{}{}

		var bytes []byte
		var err error
		if strings.TrimSpace(filePath) == "-" {
			bytes, err = ioutil.ReadAll(os.Stdin)
		} else {
			bytes, err = readFile(filePath, CertFile, KeyFile, CAFile)
		}

		if err != nil {
			return []byte{}, err
		}

		if err := yaml.Unmarshal(bytes, &currentMap); err != nil {
			return []byte{}, fmt.Errorf("failed to parse %s: %s", filePath, err)
		}
		// Merge with the previous map
		base = mergeValues(base, currentMap)
	}

	return yaml.Marshal(base)
}

// locateChartPath looks for a chart directory in known places, and returns either the full path or an error.
//
// This does not ensure that the chart is well-formed; only that the requested filename exists.
//
// Order of resolution:
// - current working directory
// - if path is absolute or begins with '.', error out here
// - chart repos in $HELM_HOME
// - URL
//
// If 'verify' is true, this will attempt to also verify the chart.
func locateChartPath(repoURL, username, password, name, version string, verify bool, keyring,
	certFile, keyFile, caFile string) (string, error) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	if fi, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return abs, err
		}
		if verify {
			if fi.IsDir() {
				return "", errors.New("cannot verify a directory")
			}
			if _, err := downloader.VerifyChart(abs, keyring); err != nil {
				return "", err
			}
		}
		return abs, nil
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, fmt.Errorf("path %q not found", name)
	}

	crepo := filepath.Join(settings.Home.Repository(), name)
	if _, err := os.Stat(crepo); err == nil {
		return filepath.Abs(crepo)
	}

	dl := downloader.ChartDownloader{
		HelmHome: settings.Home,
		Out:      os.Stdout,
		Keyring:  keyring,
		Getters:  getter.All(settings),
		Username: username,
		Password: password,
	}
	if verify {
		dl.Verify = downloader.VerifyAlways
	}
	if repoURL != "" {
		chartURL, err := repo.FindChartInAuthRepoURL(repoURL, username, password, name, version,
			certFile, keyFile, caFile, getter.All(settings))
		if err != nil {
			return "", err
		}
		name = chartURL
	}

	if _, err := os.Stat(settings.Home.Archive()); os.IsNotExist(err) {
		os.MkdirAll(settings.Home.Archive(), 0744)
	}

	filename, _, err := dl.DownloadTo(name, version, settings.Home.Archive())
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		return lname, nil
	} else if settings.Debug {
		return filename, err
	}

	return filename, fmt.Errorf("failed to download %q (hint: running `helm repo update` may help)", name)
}

func generateName(nameTemplate string) (string, error) {
	t, err := template.New("name-template").Funcs(sprig.TxtFuncMap()).Parse(nameTemplate)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	err = t.Execute(&b, nil)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

func defaultNamespace() string {
	if ns, _, err := kube.GetConfig(settings.KubeContext, settings.KubeConfig).Namespace(); err == nil {
		return ns
	}
	return "default"
}

//readFile load a file from the local directory or a remote file with a url.
func readFile(filePath, CertFile, KeyFile, CAFile string) ([]byte, error) {
	u, _ := url.Parse(filePath)
	p := getter.All(settings)

	// FIXME: maybe someone handle other protocols like ftp.
	getterConstructor, err := p.ByScheme(u.Scheme)

	if err != nil {
		return ioutil.ReadFile(filePath)
	}

	getter, err := getterConstructor(filePath, CertFile, KeyFile, CAFile)
	if err != nil {
		return []byte{}, err
	}
	data, err := getter.Get(filePath)
	return data.Bytes(), err
}
