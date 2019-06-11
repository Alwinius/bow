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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/rest"

	// Import to initialize client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	helm_env "k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/kube"
)

var (
	settings helm_env.EnvSettings
)

func newRootCmd(args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "helm",
		Short:        "The Helm package manager for Kubernetes.",
		SilenceUsage: true,
		PersistentPreRun: func(*cobra.Command, []string) {
			if settings.TLSCaCertFile == helm_env.DefaultTLSCaCert || settings.TLSCaCertFile == "" {
				settings.TLSCaCertFile = settings.Home.TLSCaCert()
			} else {
				settings.TLSCaCertFile = os.ExpandEnv(settings.TLSCaCertFile)
			}
			if settings.TLSCertFile == helm_env.DefaultTLSCert || settings.TLSCertFile == "" {
				settings.TLSCertFile = settings.Home.TLSCert()
			} else {
				settings.TLSCertFile = os.ExpandEnv(settings.TLSCertFile)
			}
			if settings.TLSKeyFile == helm_env.DefaultTLSKeyFile || settings.TLSKeyFile == "" {
				settings.TLSKeyFile = settings.Home.TLSKey()
			} else {
				settings.TLSKeyFile = os.ExpandEnv(settings.TLSKeyFile)
			}
		},
	}
	flags := cmd.PersistentFlags()

	settings.AddFlags(flags)

	out := cmd.OutOrStdout()

	cmd.AddCommand(
		newTemplateCmd(out),

		// Deprecated
	)

	flags.Parse(args)

	// set defaults from environment
	settings.Init(flags)
	return cmd
}

func init() {
	// Tell gRPC not to log to console.
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
}

func main() {
	cmd := newRootCmd(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		switch e := err.(type) {
		case pluginError:
			os.Exit(e.code)
		default:
			os.Exit(1)
		}
	}
}

func markDeprecated(cmd *cobra.Command, notice string) *cobra.Command {
	cmd.Deprecated = notice
	return cmd
}

func checkArgsLength(argsReceived int, requiredArgs ...string) error {
	expectedNum := len(requiredArgs)
	if argsReceived != expectedNum {
		arg := "arguments"
		if expectedNum == 1 {
			arg = "argument"
		}
		return fmt.Errorf("This command needs %v %s: %s", expectedNum, arg, strings.Join(requiredArgs, ", "))
	}
	return nil
}

// prettyError unwraps or rewrites certain errors to make them more user-friendly.
func prettyError(err error) error {
	// Add this check can prevent the object creation if err is nil.
	if err == nil {
		return nil
	}
	// If it's grpc's error, make it more user-friendly.
	if s, ok := status.FromError(err); ok {
		return fmt.Errorf(s.Message())
	}
	// Else return the original error.
	return err
}

// configForContext creates a Kubernetes REST client configuration for a given kubeconfig context.
func configForContext(context string, kubeconfig string) (*rest.Config, error) {
	config, err := kube.GetConfig(context, kubeconfig).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes config for context %q: %s", context, err)
	}
	return config, nil
}
