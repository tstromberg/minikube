/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/minikube/pkg/minikube/assets"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/exit"
	"k8s.io/minikube/pkg/minikube/out"
)

var addonListOutput string

// AddonListTemplate represents the addon list template
type AddonListTemplate struct {
	AddonName   string
	AddonStatus string
}

var addonsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lists all available minikube addons as well as their current statuses (enabled/disabled)",
	Long:  "Lists all available minikube addons as well as their current statuses (enabled/disabled)",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 0 {
			exit.UsageT("usage: minikube addons list")
		}

		switch strings.ToLower(addonListOutput) {
		case "list":
			printAddonsList()
		case "json":
			printAddonsJSON()
		default:
			exit.WithCodeT(exit.BadUsage, fmt.Sprintf("invalid output format: %s. Valid values: 'list', 'json'", addonListOutput))
		}
	},
}

func init() {
	addonsListCmd.Flags().StringVarP(
		&addonListOutput,
		"output",
		"o",
		"list",
		`minikube addons list --output OUTPUT. json, list`)

	AddonsCmd.AddCommand(addonsListCmd)
}

var iconFromStatus = func(addonStatus bool) string {
	if addonStatus {
		return "✅"
	}
	return "   " // because emoji indentation is different
}

var stringFromStatus = func(addonStatus bool) string {
	if addonStatus {
		return "enabled"
	}
	return "disabled"
}

var printAddonsList = func() {
	addonNames := make([]string, 0, len(assets.Addons))
	for addonName := range assets.Addons {
		addonNames = append(addonNames, addonName)
	}
	sort.Strings(addonNames)
	var tData [][]string
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Addon Name", "Profile", "Status"})
	table.SetAutoFormatHeaders(true)
	table.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
	table.SetCenterSeparator("|")
	pName := viper.GetString(config.MachineProfile)

	for _, addonName := range addonNames {
		addonBundle := assets.Addons[addonName]
		addonStatus, err := addonBundle.IsEnabled()
		if err != nil {
			out.WarningT("Unable to get addon status for {{.name}}: {{.error}}", out.V{"name": addonName, "error": err})
		}
		tData = append(tData, []string{addonName, pName, fmt.Sprintf("%s %s", stringFromStatus(addonStatus), iconFromStatus(addonStatus))})
	}

	table.AppendBulk(tData)
	table.Render()

	v, _, err := config.ListProfiles()
	if err != nil {
		glog.Errorf("list profiles returned error: %v", err)
	}
	if len(v) > 1 {
		out.T(out.Tip, "To see addons list for other profiles use: `minikube addons -p name list`")
	}
}

var printAddonsJSON = func() {
	addonNames := make([]string, 0, len(assets.Addons))
	pName := viper.GetString(config.MachineProfile)
	for addonName := range assets.Addons {
		addonNames = append(addonNames, addonName)
	}
	sort.Strings(addonNames)

	addonsMap := map[string]map[string]interface{}{}

	for _, addonName := range addonNames {
		addonBundle := assets.Addons[addonName]

		addonStatus, err := addonBundle.IsEnabled()
		if err != nil {
			glog.Errorf("Unable to get addon status for {{.name}}: {{.error}}", err)
			continue
		}

		addonsMap[addonName] = map[string]interface{}{
			"Status":  stringFromStatus(addonStatus),
			"Profile": pName,
		}
	}
	jsonString, _ := json.Marshal(addonsMap)

	out.String(string(jsonString))
}
