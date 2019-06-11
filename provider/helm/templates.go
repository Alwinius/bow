package helm

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/tiller"
	"k8s.io/helm/pkg/timeconv"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const defaultDirectoryPermission = 0755

var (
	whitespaceRegex = regexp.MustCompile(`^\s*$`)

	// defaultKubeVersion is the default value of --kube-version flag
	defaultKubeVersion = fmt.Sprintf("%s.%s", chartutil.DefaultKubeVersion.Major, chartutil.DefaultKubeVersion.Minor)
)

func Run(path string) error {
	//args[0] is chartPath
	chartPath, _ := filepath.Abs(path)
	var outputDir string
	var namespace string
	var vFiles = valueFiles{"values.yml"}
	releaseName := "whatever"

	// verify that output-dir exists if provided
	if outputDir != "" {
		_, err := os.Stat(outputDir)
		if os.IsNotExist(err) {
			return fmt.Errorf("output-dir '%s' does not exist", outputDir)
		}
	}

	if namespace == "" {
		namespace = "default"
	}
	// get combined values and create config
	rawVals, err := vals(vFiles, "", "", "")
	if err != nil {
		return err
	}
	config := &chart.Config{Raw: string(rawVals), Values: map[string]*chart.Value{}}

	if msgs := validation.IsDNS1123Subdomain(releaseName); releaseName != "" && len(msgs) > 0 {
		return fmt.Errorf("release name %s is invalid: %s", releaseName, strings.Join(msgs, ";"))
	}

	// Check chart requirements to make sure all dependencies are present in /charts
	c, err := chartutil.Load(chartPath)
	if err != nil {
		return prettyError(err)
	}

	renderOpts := renderutil.Options{
		ReleaseOptions: chartutil.ReleaseOptions{
			Name:      releaseName,
			IsInstall: !false,
			IsUpgrade: false,
			Time:      timeconv.Now(),
			Namespace: namespace,
		},
		KubeVersion: defaultKubeVersion,
	}

	renderedTemplates, err := renderutil.Render(c, config, renderOpts)
	if err != nil {
		return err
	}

	listManifests := manifest.SplitManifests(renderedTemplates)
	var manifestsToRender []manifest.Manifest

	// render all manifests in the chart
	manifestsToRender = listManifests

	for _, m := range tiller.SortByKind(manifestsToRender) {
		data := m.Content
		b := filepath.Base(m.Name)
		if b == "NOTES.txt" {
			continue
		}
		if strings.HasPrefix(b, "_") {
			continue
		}

		if outputDir != "" {
			// blank template after execution
			if whitespaceRegex.MatchString(data) {
				continue
			}
			err = writeToFile(outputDir, m.Name, data)
			if err != nil {
				return err
			}
			continue
		}
		fmt.Printf("---\n# Source: %s\n", m.Name)
		fmt.Println(data)
	}
	return nil
}

// write the <data> to <output-dir>/<name>
func writeToFile(outputDir string, name string, data string) error {
	outfileName := strings.Join([]string{outputDir, name}, string(filepath.Separator))

	err := ensureDirectoryForFile(outfileName)
	if err != nil {
		return err
	}

	f, err := os.Create(outfileName)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("---\n# Source: %s\n%s", name, data))

	if err != nil {
		return err
	}

	fmt.Printf("wrote %s\n", outfileName)
	return nil
}

// check if the directory exists to create file. creates if don't exists
func ensureDirectoryForFile(file string) error {
	baseDir := path.Dir(file)
	_, err := os.Stat(baseDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.MkdirAll(baseDir, defaultDirectoryPermission)
}
