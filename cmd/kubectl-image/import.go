// Copyright 2020 The Shipwright Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/shipwright-io/image/cmd/kubectl-image/static"
	itagcli "github.com/shipwright-io/image/infra/images/v1beta1/gen/clientset/versioned"
	"github.com/shipwright-io/image/services"
)

func init() {
	imageimport.Flags().StringP("namespace", "n", "", "namespace to use")
	imageimport.Flags().StringP("source", "s", "", "image source for the import")
	imageimport.Flags().Bool("mirror", false, "mirror the image")
	imageimport.Flags().Bool("insecure-source", false, "skip tls check for the remote registry")
}

var imageimport = &cobra.Command{
	Use:     "import --source docker.io/library/centos -n <namespace> <image name>",
	Short:   "Imports an Image",
	Long:    static.Text["import_help_header"],
	Example: static.Text["import_help_examples"],
	RunE: func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if len(args) != 1 {
			return fmt.Errorf("provide an image name")
		}

		ns, err := namespace(c)
		if err != nil {
			return err
		}

		source, err := c.Flags().GetString("source")
		if err != nil {
			return err
		}

		mirror, err := c.Flags().GetBool("mirror")
		if err != nil {
			return err
		}

		ins, err := c.Flags().GetBool("insecure-source")
		if err != nil {
			return err
		}

		tisvc, err := createImageImportService()
		if err != nil {
			return err
		}

		opts := services.ImportOpts{
			Namespace: ns,
			Image:     args[0],
			Source:    source,
			Mirror:    &mirror,
			Insecure:  &ins,
		}

		ti, err := tisvc.NewImport(ctx, opts)
		if err != nil {
			return err
		}

		fmt.Printf("new image import request created: %s/%s\n", ns, ti.Name)
		return nil
	},
}

func createImageImportService() (*services.ImageImport, error) {
	cfgpath := os.Getenv("KUBECONFIG")

	config, err := clientcmd.BuildConfigFromFlags("", cfgpath)
	if err != nil {
		return nil, fmt.Errorf("error building config: %s", err)
	}

	tagcli, err := itagcli.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return services.NewImageImport(nil, tagcli, nil), nil
}
