package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	homePath     string
	dataDir      string
	backend      string
	app          string
	cosmosSdk    bool
	tendermint   bool
	blocks       int64
	keepVersions int64
	keepEvery    int64
	batch        uint64
	parallel     uint64
	profile      string
	modules      []string
	appName      = "cosmos-pruner"
)

// NewRootCmd returns the root command for relayer.
func NewRootCmd() *cobra.Command {
	// RootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   appName,
		Short: "cosmprund is meant to prune data base history from a cosmos application, avoiding needing to state sync every couple amount of weeks",
	}

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return nil
	}

	// --pruning flag
	rootCmd.PersistentFlags().StringVar(&profile, "pruning", "default", "pruning profile")
	if err := viper.BindPFlag("pruning", rootCmd.PersistentFlags().Lookup("pruning")); err != nil {
		panic(err)
	}

	// --min-retain-blocks flag
	rootCmd.PersistentFlags().
		Int64Var(&blocks, "min-retain-blocks", -1, "set the amount of tendermint blocks to be kept (default=0)")
	if err := viper.BindPFlag("min-retain-blocks", rootCmd.PersistentFlags().Lookup("min-retain-blocks")); err != nil {
		panic(err)
	}

	// --pruning-keep-recent flag
	rootCmd.PersistentFlags().
		Int64Var(&keepVersions, "pruning-keep-recent", -1, "set the amount of versions to keep in the application store (default=400000)")
	if err := viper.BindPFlag("pruning-keep-recent", rootCmd.PersistentFlags().Lookup("pruning-keep-recent")); err != nil {
		panic(err)
	}

	// --pruning-keep-every flag
	rootCmd.PersistentFlags().
		Int64Var(&keepEvery, "pruning-keep-every", -1, "set the version interval to be kept in the application store (default=None)")
	if err := viper.BindPFlag("pruning-keep-every", rootCmd.PersistentFlags().Lookup("pruning-keep-every")); err != nil {
		panic(err)
	}

	// --batch flag
	rootCmd.PersistentFlags().
		Uint64Var(&batch, "batch", 10000, "set the amount of versions to be pruned in one batch (default=10000)")
	if err := viper.BindPFlag("batch", rootCmd.PersistentFlags().Lookup("batch")); err != nil {
		panic(err)
	}

	// --parallel-limit flag
	rootCmd.PersistentFlags().
		Uint64Var(&parallel, "parallel-limit", 16, "set the limit of parallel go routines to be running at the same time (default=16)")
	if err := viper.BindPFlag("parallel-limit", rootCmd.PersistentFlags().Lookup("parallel-limit")); err != nil {
		panic(err)
	}

	// --modules flag
	rootCmd.PersistentFlags().
		StringSliceVar(&modules, "modules", []string{}, "extra modules to be pruned in format: \"module_name,module_name\"")
	if err := viper.BindPFlag("modules", rootCmd.PersistentFlags().Lookup("modules")); err != nil {
		panic(err)
	}

	// --backend flag
	rootCmd.PersistentFlags().
		StringVar(&backend, "backend", "goleveldb", "set the type of db being used(default=goleveldb)")
		//todo add list of dbs to comment
	if err := viper.BindPFlag("backend", rootCmd.PersistentFlags().Lookup("backend")); err != nil {
		panic(err)
	}

	// --app flag
	rootCmd.PersistentFlags().StringVar(&app, "app", "", "set the app you are pruning (supported apps: bandchain)")
	if err := viper.BindPFlag("app", rootCmd.PersistentFlags().Lookup("app")); err != nil {
		panic(err)
	}

	// --cosmos-sdk flag
	rootCmd.PersistentFlags().
		BoolVar(&cosmosSdk, "cosmos-sdk", true, "set to false if using only with tendermint (default true)")
	if err := viper.BindPFlag("cosmos-sdk", rootCmd.PersistentFlags().Lookup("cosmos-sdk")); err != nil {
		panic(err)
	}

	// --tendermint flag
	rootCmd.PersistentFlags().
		BoolVar(&tendermint, "tendermint", true, "set to false you dont want to prune tendermint data(default true)")
	if err := viper.BindPFlag("tendermint", rootCmd.PersistentFlags().Lookup("tendermint")); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(
		pruneCmd(),
		compactCmd(),
	)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.EnableCommandSorting = false

	rootCmd := NewRootCmd()
	rootCmd.SilenceUsage = true
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
