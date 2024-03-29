package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"kore-on/pkg/logger"
	"kore-on/pkg/utils"
	"os"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/execute/measure"
	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/apenella/go-ansible/pkg/stdoutcallback/results"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Commands structure
type strTestCmd struct {
	dryRun         bool
	verbose        bool
	step           bool
	inventory      string
	tags           string
	playbookFiles  []string
	privateKey     string
	user           string
	extravars      map[string]interface{}
	addonExtravars map[string]interface{}
	result         map[string]interface{}
}

func TestCmd() *cobra.Command {
	test := &strTestCmd{}

	cmd := &cobra.Command{
		Use:          "test [flags]",
		Short:        "Install kubernetes cluster, registry",
		Long:         "",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return test.run()
		},
	}

	test.tags = ""
	test.inventory = "./internal/playbooks/koreon-playbook/inventory/inventory.ini"
	test.playbookFiles = []string{
		"./internal/playbooks/koreon-playbook/z-test-cri.yaml",
	}

	f := cmd.Flags()
	f.BoolVarP(&test.verbose, "verbose", "v", false, "verbose")
	f.BoolVarP(&test.step, "step", "", false, "step")
	f.BoolVarP(&test.dryRun, "dry-run", "d", false, "dryRun")
	f.StringVarP(&test.inventory, "inventory", "i", test.inventory, "Specify ansible playbook inventory")
	f.StringVarP(&test.privateKey, "private-key", "p", "", "Specify ssh key path")
	f.StringVarP(&test.user, "user", "u", "", "login user")
	f.StringVar(&test.tags, "tags", "", "Ansible options tags")

	return cmd
}

func (c *strTestCmd) run() error {
	koreOnConfigFileName := viper.GetString("KoreOn.KoreOnConfigFile")
	koreOnConfigFilePath := utils.IskoreOnConfigFilePath(koreOnConfigFileName)
	koreonToml, value := utils.ValidateKoreonTomlConfig(koreOnConfigFilePath, "create")

	if value {
		// Prompt user for more input
		// id := utils.InputPrompt("# Enter the username for the private registry.\nusername:")
		// koreonToml.KoreOn.HelmCubeRepoID = base64.StdEncoding.EncodeToString([]byte(id))

		// pw := utils.SensitivePrompt("# Enter the password for the private registry.\npassword:")
		// koreonToml.KoreOn.HelmCubeRepoPW = base64.StdEncoding.EncodeToString([]byte(pw))

		b, err := json.Marshal(koreonToml)
		if err != nil {
			logger.Fatal(err)
			os.Exit(1)
		}
		if err := json.Unmarshal(b, &c.extravars); err != nil {
			logger.Fatal(err.Error())
			os.Exit(1)
		}
	}

	addonConfigFileName := viper.GetString("Addon.AddonConfigFile")
	addonPath := utils.IskoreOnConfigFilePath(addonConfigFileName)
	addonToml, err := utils.GetAddonTomlConfig(addonPath)
	if err != nil {
		logger.Fatal(err)
	} else {
		// Prompt user for more input
		// if addonToml.Apps.CsiDriverNfs.Install {
		// 	id := utils.InputPrompt("# Enter the username for the private registry.\nusername:")
		// 	addonToml.Apps.CsiDriverNfs.ChartRefID = base64.StdEncoding.EncodeToString([]byte(id))

		// 	pw := utils.SensitivePrompt("# Enter the password for the private registry.\npassword:")
		// 	addonToml.Apps.CsiDriverNfs.ChartRefPW = base64.StdEncoding.EncodeToString([]byte(pw))
		// }

		if addonToml.Addon.AddonDataDir == "" {
			addonToml.Addon.AddonDataDir = "/data/addon"
		}

		b, err := json.Marshal(addonToml)
		if err != nil {
			logger.Fatal(err)
		}
		if err := json.Unmarshal(b, &c.addonExtravars); err != nil {
			logger.Fatal(err.Error())
		}

		result := make(map[string]interface{})
		for k, v := range c.extravars {
			if _, ok := c.extravars[k]; ok {
				result[k] = v
			}
		}
		for k, v := range c.addonExtravars {
			if _, ok := c.addonExtravars[k]; ok {
				result[k] = v
			}
		}
		c.result = result
	}

	if len(c.playbookFiles) < 1 {
		return fmt.Errorf("[ERROR]: %s", "To run ansible-playbook playbook file path must be specified")
	}

	if len(c.inventory) < 1 {
		return fmt.Errorf("[ERROR]: %s", "To run ansible-playbook an inventory must be specified")
	}

	// if len(c.privateKey) < 1 {
	// 	if len(koreonToml.NodePool.Security.PrivateKeyPath) > 0 {
	// 		c.privateKey = koreonToml.NodePool.Security.PrivateKeyPath
	// 	} else {
	// 		return fmt.Errorf("[ERROR]: %s", "To run ansible-playbook an privateKey must be specified")
	// 	}
	// }

	// if len(c.user) < 1 {
	// 	if len(koreonToml.NodePool.Security.SSHUserID) > 0 {
	// 		c.user = koreonToml.NodePool.Security.SSHUserID
	// 	} else {
	// 		return fmt.Errorf("[ERROR]: %s", "To run ansible-playbook an ssh login user must be specified")
	// 	}
	// }

	ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{
		PrivateKey: c.privateKey,
		User:       c.user,
	}

	ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
		Inventory: c.inventory,
		Verbose:   c.verbose,
		Tags:      c.tags,
		ExtraVars: c.result,
	}

	executorTimeMeasurement := measure.NewExecutorTimeMeasurement(
		execute.NewDefaultExecute(
			execute.WithEnvVar("ANSIBLE_FORCE_COLOR", "true"),
			execute.WithTransformers(
				utils.OutputColored(),
				results.Prepend("cobra-cmd-ansibleplaybook example"),
				// results.LogFormat(results.DefaultLogFormatLayout, results.Now),
			),
		),
		measure.WithShowDuration(),
	)

	playbook := &playbook.AnsiblePlaybookCmd{
		Playbooks:         c.playbookFiles,
		ConnectionOptions: ansiblePlaybookConnectionOptions,
		Options:           ansiblePlaybookOptions,
		Exec:              executorTimeMeasurement,
	}

	options.AnsibleForceColor()

	err = playbook.Run(context.TODO())
	if err != nil {
		return err
	}

	return nil
}
