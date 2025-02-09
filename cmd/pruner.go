package cmd

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/neilotoole/errgroup"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb/opt"

	db "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/state"
	tmstore "github.com/cometbft/cometbft/store"

	"cosmossdk.io/log"
	"cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	dbm "github.com/cosmos/cosmos-db"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	icahosttypes "github.com/cosmos/ibc-go/v8/modules/apps/27-interchain-accounts/host/types"
	ibcfeetypes "github.com/cosmos/ibc-go/v8/modules/apps/29-fee/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/exported"

	"github.com/binaryholdings/cosmos-pruner/internal/rootmulti"
)

type pruningProfile struct {
	name         string
	blocks       uint64
	keepVersions uint64
}

var (
	PruningProfiles = map[string]pruningProfile{
		"default":    {"default", 0, 400000},
		"nothing":    {"nothing", 0, 0},
		"everything": {"everything", 0, 10},
		"emitter":    {"emitter", 100000, 100},
		"rest-light": {"rest-light", 600000, 100000},
		"rest-heavy": {"rest-heavy", 0, 400000},
		"peer":       {"peer", 0, 100},
		"seed":       {"seed", 100000, 100},
		"sentry":     {"sentry", 300000, 100},
		"validator":  {"validator", 100000, 100},
	}
)

// load db
// load app store and prune
// if immutable tree is not deletable we should import and export current state
func pruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "prune data from the application store and block store",
		RunE: func(cmd *cobra.Command, args []string) error {

			if profile != "custom" {
				if _, ok := PruningProfiles[profile]; !ok {
					return fmt.Errorf("Invalid Pruning Profile")
				}
				if !cmd.Flag("min-retain-blocks").Changed && cmd.Flag("pruning").Changed {
					blocks = PruningProfiles[profile].blocks
				}
				if !cmd.Flag("pruning-keep-recent").Changed {
					keepVersions = PruningProfiles[profile].keepVersions
				}
			}

			fmt.Println("app:", app)
			fmt.Println("profile:", profile)
			fmt.Println("pruning-keep-recent:", keepVersions)
			fmt.Println("min-retain-blocks:", blocks)
			fmt.Println("batch:", batch)
			fmt.Println("parallel-limit:", parallel)

			ctx := cmd.Context()
			errs, _ := errgroup.WithContext(ctx)
			var err error

			if tendermint {
				errs.Go(func() error {
					if err = pruneTMData(homePath); err != nil {
						return err
					}

					return nil
				})
			}

			if cosmosSdk {
				if err = pruneAppState(homePath); err != nil {
					return err
				}
			}

			return errs.Wait()
		},
	}

	return cmd
}

func compactCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "compact",
		Short: "compact data from the application store and block store",
		RunE: func(cmd *cobra.Command, args []string) error {

			dbDir := rootify(dataDir, homePath)

			o := opt.Options{
				DisableSeeksCompaction: true,
			}

			if cosmosSdk {
				// Get BlockStore
				appDB, err := db.NewGoLevelDBWithOpts("application", dbDir, &o)
				if err != nil {
					return err
				}

				fmt.Println("compacting application state")
				if err := appDB.Compact(nil, nil); err != nil {
					return err
				}

				appDB.Close()
			}

			if tendermint {
				// Get BlockStore
				blockStoreDB, err := db.NewGoLevelDBWithOpts("blockstore", dbDir, &o)
				if err != nil {
					return err
				}

				fmt.Println("compacting block store")
				if err := blockStoreDB.Compact(nil, nil); err != nil {
					return err
				}

				// Get StateStore
				stateDB, err := db.NewGoLevelDBWithOpts("state", dbDir, &o)
				if err != nil {
					return err
				}

				fmt.Println("compacting state store")
				if err := stateDB.Compact(nil, nil); err != nil {
					return err
				}

				blockStoreDB.Close()
				stateDB.Close()
			}

			return nil
		},
	}

	return cmd
}

func pruneAppState(home string) error {
	fmt.Println("pruning application state")

	// Get application store
	dbDir := rootify(dataDir, home)
	appDB, err := dbm.NewDB("application", dbm.GoLevelDBBackend, dbDir)
	if err != nil {
		return err
	}

	// only mount keys from core sdk
	// todo allow for other keys to be mounted
	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey,
		banktypes.StoreKey,
		stakingtypes.StoreKey,
		crisistypes.StoreKey,
		minttypes.StoreKey,
		distrtypes.StoreKey,
		slashingtypes.StoreKey,
		govtypes.StoreKey,
		paramstypes.StoreKey,
		consensusparamtypes.StoreKey,
		ibcexported.StoreKey,
		upgradetypes.StoreKey,
		evidencetypes.StoreKey,
		ibctransfertypes.StoreKey,
		capabilitytypes.StoreKey,
		feegrant.StoreKey,
		authzkeeper.StoreKey,
		ibcfeetypes.StoreKey,
		icahosttypes.StoreKey,
	)

	if app == "bandchain" {
		bandchainKeys := storetypes.NewKVStoreKeys(
			"oracle",
			"globalfee",
			"restake",
			"feeds",
			"bandtss",
			"tss",
			"rollingseed",
			"tunnel",
		)

		for key, value := range bandchainKeys {
			keys[key] = value
		}
	}

	extraKeys := storetypes.NewKVStoreKeys(modules...)

	for key, value := range extraKeys {
		keys[key] = value
	}

	wg := sync.WaitGroup{}
	var pruneErr error

	guard := make(chan struct{}, parallel)
	for _, value := range keys {
		guard <- struct{}{}
		wg.Add(1)
		go func(value *storetypes.KVStoreKey) {
			err := func(value *storetypes.KVStoreKey) error {
				appStore := rootmulti.NewStore(appDB, log.NewNopLogger(), metrics.NewNoOpMetrics())
				appStore.MountStoreWithDB(value, storetypes.StoreTypeIAVL, nil)
				err = appStore.LoadLatestVersion()
				if err != nil {
					return err
				}

				latestHeight := rootmulti.GetLatestVersion(appDB)
				// valid heights should be greater than 0.
				if latestHeight <= 0 {
					return fmt.Errorf("the database has no valid heights to prune, the latest height: %v", latestHeight)
				}

				pruningHeight := latestHeight - int64(keepVersions)

				fmt.Printf("pruning store: %+v to %+v/%+v\n", value.Name(), pruningHeight, latestHeight)

				err := appStore.PruneStores(batch, pruningHeight)
				if err != nil {
					fmt.Println("error pruning store:", value.Name(), err)
					return err
				}

				fmt.Println("finished pruning store:", value.Name())

				return nil
			}(value)

			if err != nil {
				pruneErr = err
			}
			<-guard
			defer wg.Done()
		}(value)
	}
	wg.Wait()

	if pruneErr != nil {
		return pruneErr
	}

	appDB.Close()

	// Compacting db
	fmt.Println("compacting application state")

	o := opt.Options{
		DisableSeeksCompaction: true,
	}

	goDB, err := db.NewGoLevelDBWithOpts("application", dbDir, &o)
	if err != nil {
		return err
	}

	if err := goDB.Compact(nil, nil); err != nil {
		return err
	}

	goDB.Close()

	return nil
}

// pruneTMData prunes the tendermint blocks and state based on the amount of blocks to keep
func pruneTMData(home string) error {
	dbDir := rootify(dataDir, home)

	o := opt.Options{
		DisableSeeksCompaction: true,
	}

	// Get BlockStore
	blockStoreDB, err := db.NewGoLevelDBWithOpts("blockstore", dbDir, &o)
	if err != nil {
		return err
	}
	blockStore := tmstore.NewBlockStore(blockStoreDB)

	// Get StateStore
	stateDB, err := db.NewGoLevelDBWithOpts("state", dbDir, &o)
	if err != nil {
		return err
	}

	stateStore := state.NewStore(stateDB, state.StoreOptions{
		DiscardABCIResponses: true,
	})

	if blocks == 0 {
		return nil
	}
	if blocks < 100000 {
		return fmt.Errorf("Your min-retain-blocks %+v is lower than the minimum 100000", blocks)
	}

	pruneHeight := blockStore.Height() - int64(blocks)

	// prune block store
	base := blockStore.Base()
	if base < pruneHeight {
		fmt.Printf("pruning block from %+v to %+v/%+v\n", base, pruneHeight, blockStore.Height())

		state, err := stateStore.LoadFromDBOrGenesisFile("")
		if err != nil {
			return err
		}

		fmt.Println("pruning block store")
		_, evidenceHeight, err := blockStore.PruneBlocks(pruneHeight, state)
		if err != nil {
			return err
		}

		fmt.Println("pruning state store")
		err = stateStore.PruneStates(base, pruneHeight, evidenceHeight)
		if err != nil {
			return err
		}
	}

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		fmt.Println("compacting block store")
		if err := blockStoreDB.Compact(nil, nil); err != nil {
			fmt.Printf("compacting block store failed: %+v", err)
		}
	}()

	go func() {
		defer wg.Done()

		fmt.Println("compacting state store")
		if err := stateDB.Compact(nil, nil); err != nil {
			fmt.Printf("compacting state store failed: %+v", err)
		}
	}()

	wg.Wait()

	stateDB.Close()
	blockStore.Close()

	return nil
}

// Utils
func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
