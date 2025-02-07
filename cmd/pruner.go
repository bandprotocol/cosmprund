package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/neilotoole/errgroup"
	"github.com/spf13/cobra"
	"github.com/syndtr/goleveldb/leveldb/opt"

	db "github.com/cometbft/cometbft-db"
	tmstore "github.com/cometbft/cometbft/store"
)

type pruningProfile struct {
	name         string
	blocks       uint64
	keepVersions uint64
	keepEvery    uint64
}

var (
	PruningProfiles = map[string]pruningProfile{
		"default":    {"default", 0, 400000, 100},
		"nothing":    {"nothing", 0, 0, 1},
		"everything": {"everything", 0, 10, 0},
		"emitter":    {"emitter", 100000, 100, 0},
		"rest-light": {"rest-light", 600000, 100000, 0},
		"rest-heavy": {"rest-heavy", 0, 400000, 1000},
		"peer":       {"peer", 0, 100, 30000},
		"seed":       {"seed", 100000, 100, 0},
		"sentry":     {"sentry", 300000, 100, 0},
		"validator":  {"validator", 100000, 100, 0},
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
				if !cmd.Flag("pruning-keep-every").Changed {
					keepEvery = PruningProfiles[profile].keepEvery
				}
			}

			fmt.Println("app:", app)
			fmt.Println("profile:", profile)
			fmt.Println("pruning-keep-every:", keepEvery)
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
			}

			return nil
		},
	}

	return cmd
}

func pruneAppState(home string) error {
	dbDir := rootify(dataDir, home)

	o := opt.Options{
		DisableSeeksCompaction: true,
	}

	// Get BlockStore
	appDB, err := db.NewGoLevelDBWithOpts("application", dbDir, &o)
	if err != nil {
		return err
	}

	// fmt.Println("pruning application state")

	// // only mount keys from core sdk
	// // todo allow for other keys to be mounted
	// keys := storetypes.NewKVStoreKeys(
	// 	authtypes.StoreKey,
	// 	banktypes.StoreKey,
	// 	stakingtypes.StoreKey,
	// 	crisistypes.StoreKey,
	// 	minttypes.StoreKey,
	// 	distrtypes.StoreKey,
	// 	slashingtypes.StoreKey,
	// 	govtypes.StoreKey,
	// 	paramstypes.StoreKey,
	// 	consensusparamtypes.StoreKey,
	// 	ibcexported.StoreKey,
	// 	upgradetypes.StoreKey,
	// 	evidencetypes.StoreKey,
	// 	ibctransfertypes.StoreKey,
	// 	capabilitytypes.StoreKey,
	// 	feegrant.StoreKey,
	// 	authzkeeper.StoreKey,
	// 	ibcfeetypes.StoreKey,
	// 	icahosttypes.StoreKey,
	// )

	// if app == "bandchain" {
	// 	bandchainKeys := storetypes.NewKVStoreKeys(
	// 		"oracle", // oracletypes.StoreKey,
	// 		"globalfee",
	// 		"restake",
	// 		"feeds",
	// 		"bandtss",
	// 		"tss",
	// 		"rollingseed",
	// 		"tunnel",
	// 	)

	// 	for key, value := range bandchainKeys {
	// 		keys[key] = value
	// 	}
	// }

	// extraKeys := storetypes.NewKVStoreKeys(modules...)

	// for key, value := range extraKeys {
	// 	keys[key] = value
	// }

	// wg := sync.WaitGroup{}
	// var pruneErr error

	// guard := make(chan struct{}, parallel)
	// for _, value := range keys {
	// 	guard <- struct{}{}
	// 	wg.Add(1)
	// 	go func(value *storetypes.KVStoreKey) {
	// 		err := func(value *storetypes.KVStoreKey) error {
	// 			appStore := rootmulti.NewStore(appDB, log.NewNopLogger())
	// 			appStore.MountStoreWithDB(value, storetypes.StoreTypeIAVL, nil)
	// 			err = appStore.LoadLatestVersion()
	// 			if err != nil {
	// 				return err
	// 			}

	// 			versions := appStore.GetAllVersions()

	// 			v64 := make([]int64, 0)
	// 			for i := 0; i < len(versions); i++ {
	// 				if (keepEvery == 0 || versions[i]%int(keepEvery) != 0) &&
	// 					versions[i] <= versions[len(versions)-1]-int(keepVersions) {
	// 					v64 = append(v64, int64(versions[i]))
	// 				}
	// 			}

	// 			pruneHeights := v64[:]

	// 			fmt.Printf("pruning store: %+v (%d/%d)\n", value.Name(), len(v64), len(versions))

	// 			err := appStore.PruneStores(int(batch), false, pruneHeights)
	// 			if err != nil {
	// 				fmt.Println("error pruning store:", value.Name(), err)
	// 				return err
	// 			}

	// 			fmt.Println("finished pruning store:", value.Name())

	// 			return nil
	// 		}(value)

	// 		if err != nil {
	// 			pruneErr = err
	// 		}
	// 		<-guard
	// 		defer wg.Done()
	// 	}(value)
	// }
	// wg.Wait()

	// if pruneErr != nil {
	// 	return pruneErr
	// }

	fmt.Println("compacting application state")
	if err := appDB.Compact(nil, nil); err != nil {
		return err
	}

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

	// stateStore := state.NewStore(stateDB, state.StoreOptions{
	// 	DiscardABCIResponses: true,
	// })

	// base := blockStore.Base()

	if blocks == 0 {
		return nil
	}
	if blocks < 100000 {
		return fmt.Errorf("Your min-retain-blocks %+v is lower than the minimum 100000", blocks)
	}

	// pruneHeight := blockStore.Height() - int64(blocks)

	errs, _ := errgroup.WithContext(context.Background())
	errs.Go(func() error {
		// // prune block store
		// fmt.Println("pruning block store")
		// if base < pruneHeight {
		// 	blocks, err = blockStore.PruneBlocks(pruneHeight)
		// 	if err != nil {
		// 		return err
		// 	}
		// }

		fmt.Println("compacting block store")
		if err := blockStoreDB.Compact(nil, nil); err != nil {
			return err
		}

		return nil
	})

	// fmt.Println("pruning state store")
	// if base < pruneHeight {
	// 	err = stateStore.PruneStates(base, pruneHeight)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	fmt.Println("compacting state store")
	if err := stateDB.Compact(nil, nil); err != nil {
		return err
	}

	if err := errs.Wait(); err != nil {
		return err
	}

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
