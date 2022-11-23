package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/cosmos/cosmos-sdk/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v2/modules/apps/transfer/types"
	ibchost "github.com/cosmos/ibc-go/v2/modules/core/24-host"
	"github.com/neilotoole/errgroup"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tendermint/tendermint/state"
	tmstore "github.com/tendermint/tendermint/store"
	db "github.com/tendermint/tm-db"

	"github.com/binaryholdings/cosmos-pruner/internal/rootmulti"
)

type pruningProfile struct {
	name         string
	blocks       int64
	keepVersions int64
	keepEvery    int64
}

var (
	PruningProfiles = map[string]pruningProfile{
		"default":    pruningProfile{"default", 300000, 500000, 0},
		"emitter":    pruningProfile{"emitter", 300000, 100, 0},
		"rest-light": pruningProfile{"rest-light", 600000, 100000, 0},
		"rest-heavy": pruningProfile{"rest-heavy", 0, 400000, 1000},
		"peer":       pruningProfile{"peer", 0, 100, 30000},
		"seed":       pruningProfile{"seed", 300000, 100, 0},
		"sentry":     pruningProfile{"sentry", 600000, 100, 0},
		"validator":  pruningProfile{"validator", 600000, 100, 0},
	}
)

// load db
// load app store and prune
// if immutable tree is not deletable we should import and export current state

func pruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune [path_to_home]",
		Short: "prune data from the application store and block store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			if _, ok := PruningProfiles[profile]; !ok {
				return fmt.Errorf("Invalid Pruning Profile")
			}
			if blocks < 0 {
				blocks = PruningProfiles[profile].blocks
			}
			if keepVersions < 0 {
				keepVersions = PruningProfiles[profile].keepVersions
			}
			if keepEvery < 0 {
				keepEvery = PruningProfiles[profile].keepEvery
			}

			fmt.Println("profile: ", profile)
			fmt.Println("pruning-keep-every: ", keepEvery)
			fmt.Println("pruning-keep-recent: ", keepVersions)
			fmt.Println("min-retain-blocks: ", blocks)
			fmt.Println("batch: ", batch)
			fmt.Println("parallel-limit: ", parallel)

			var err error
			if tendermint {
				if err = pruneTMData(args[0]); err != nil {
					return err
				}
			}

			if cosmosSdk {
				err = pruneAppState(args[0])
				if err != nil {
					return err
				}
				return nil

			}

			return nil
		},
	}
	return cmd
}

func compactCmd() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "compact [path_to_home]",
		Short: "compact data from the application store and block store",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			dbDir := rootify(dataDir, args[0])

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
				if err := appDB.ForceCompact(nil, nil); err != nil {
					return err
				}
			}

			if tendermint {
				// Get BlockStore
				blockStoreDB, err := db.NewGoLevelDBWithOpts("blockstore", dbDir, &o)
				if err != nil {
					return err
				}

				fmt.Println("compacting block store")
				if err := blockStoreDB.ForceCompact(nil, nil); err != nil {
					return err
				}

				// Get StateStore
				stateDB, err := db.NewGoLevelDBWithOpts("state", dbDir, &o)
				if err != nil {
					return err
				}

				fmt.Println("compacting state store")
				if err := stateDB.ForceCompact(nil, nil); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd
}

func pruneAppState(home string) error {

	// this has the potential to expand size, should just use state sync
	// dbType := db.BackendType(backend)

	dbDir := rootify(dataDir, home)

	o := opt.Options{
		DisableSeeksCompaction: true,
	}

	// Get BlockStore
	appDB, err := db.NewGoLevelDBWithOpts("application", dbDir, &o)
	if err != nil {
		return err
	}

	//TODO: need to get all versions in the store, setting randomly is too slow
	fmt.Println("pruning application state")

	// only mount keys from core sdk
	// todo allow for other keys to be mounted
	keys := types.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, ibchost.StoreKey, upgradetypes.StoreKey,
		evidencetypes.StoreKey, ibctransfertypes.StoreKey, capabilitytypes.StoreKey,
	)

	if app == "bandchain" {
		bandchainKeys := types.NewKVStoreKeys(
			"feegrant", // feegrant.StoreKey,
			"authz",    // authzkeeper.StoreKey,
			"oracle",   // oracletypes.StoreKey,
			"icahost",
		)

		for key, value := range bandchainKeys {
			keys[key] = value
		}
	}

	extraKeys := types.NewKVStoreKeys(modules...)

	for key, value := range extraKeys {
		keys[key] = value
	}

	wg := sync.WaitGroup{}
	var prune_err error

	guard := make(chan struct{}, parallel)
	for _, value := range keys {
		guard <- struct{}{}
		wg.Add(1)
		go func(value *types.KVStoreKey) {
			err := func(value *types.KVStoreKey) error {
				// TODO: cleanup app state
				appStore := rootmulti.NewStore(appDB)
				appStore.MountStoreWithDB(value, sdk.StoreTypeIAVL, nil)
				err = appStore.LoadLatestVersion()
				if err != nil {
					return err
				}

				versions := appStore.GetAllVersions()
				if int(keepVersions) >= len(versions) {
					return nil
				}

				v64 := make([]int64, 0)
				for i := 0; i < len(versions); i++ {
					if (keepEvery == 0 || versions[i]%int(keepEvery) != 0) &&
						versions[i] <= versions[len(versions)-1]-int(keepVersions) {
						v64 = append(v64, int64(versions[i]))
					}
				}

				appStore.PruneHeights = v64[:]

				appStore.PruneStores(int(batch))

				return nil
			}(value)

			if err != nil {
				prune_err = err
			}
			<-guard
			defer wg.Done()
		}(value)
	}
	wg.Wait()

	if prune_err != nil {
		return prune_err
	}

	fmt.Println("compacting application state")
	if err := appDB.ForceCompact(nil, nil); err != nil {
		return err
	}

	//create a new app store
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

	stateStore := state.NewStore(stateDB)

	base := blockStore.Base()

	if blocks == 0 {
		return nil
	}
	if blocks < 300000 {
		return fmt.Errorf("Your min-retain-blocks %+v is lower than the minimum 300000", blocks)
	}

	pruneHeight := blockStore.Height() - int64(blocks)

	errs, _ := errgroup.WithContext(context.Background())
	errs.Go(func() error {
		fmt.Println("pruning block store")
		// prune block store
		if base < pruneHeight {
			tmp_blocks, err := blockStore.PruneBlocks(pruneHeight)
			blocks = int64(tmp_blocks)
			if err != nil {
				return err
			}
		}

		fmt.Println("compacting block store")
		if err := blockStoreDB.ForceCompact(nil, nil); err != nil {
			return err
		}

		return nil
	})

	fmt.Println("pruning state store")
	// prune state store
	if base < pruneHeight {
		err = stateStore.PruneStates(base, pruneHeight)
		if err != nil {
			return err
		}
	}

	fmt.Println("compacting state store")
	if err := stateDB.ForceCompact(nil, nil); err != nil {
		return err
	}

	return nil
}

// Utils

func rootify(path, root string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
