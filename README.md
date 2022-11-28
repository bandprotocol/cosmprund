# Cosmos-Pruner

The goal of this project is to be able to prune a tendermint data base of blocks and an Cosmos-sdk application DB of all but the last X versions. This will allow people to not have to state sync every x days. 

This tool works with a subset of modules. While an application may have modules outside the scope of this tool , this tool will prune the default sdk module, and band added module.

# Disclaimer

This tool is developed to be used internally for Band node but it may be useful for other chains node as well. If you want to use this tool, use at your own risk.

## WARNING

Due to inefficiencies of iavl and the simple approach of this tool, it can take ages to prune the data of a large node.  

We are working on integrating this natively into the Cosmos-sdk and Tendermint

## How to use

Cosmprund works of a home directory that has the same structure of a normal cosmos-sdk/tendermint node. By default it will prune follow the config from app.toml in your home path.

> Note: Application pruning can take a very long time dependent on the size of the db. 

```
# clone & build cosmprund repo
git clone https://github.com/bandprotocol/cosmprund.git
cd cosmprund
make install

# stop daemon/cosmovisor
sudo systemctl stop bandd

# run pruning using config from app.toml
cosmos-pruner prune

# run compacting
cosmos-pruner compact

# run pruning with params
cosmos-pruner prune --home ~/.band --pruning validator --app=bandchain

# run compacting with params
cosmos-pruner compact --home ~/.band
```

Flags: 

- `home`: path to directory for config and data (default=~/.band)
- `app`: the application you want to prune, outside the sdk default modules. See `Supported Apps`
- `cosmos-sdk`: If pruning a non cosmos-sdk chain, like Nomic, you only want to use tendermint pruning or if you want to only prune tendermint block & state as this is generally large on machines(Default true)
- `tendermint`: If the user wants to only prune application data they can disable pruning of tendermint data. (Default true)
- `min-retain-blocks`: set the amount of tendermint blocks to be kept (default=300000)
- `pruning-keep-recent`: set the amount of versions to keep in the application store (default=500000)
- `pruning-keep-every`: set the version interval to be kept in the application store (default=None)
- `pruning`: pruning profile (default "default")
- `batch`: set the amount of versions to be pruned in one batch (default=10000)
- `parallel-limit`: set the limit of parallel go routines to be running at the same time (default=16)
- `modules`: extra modules to be pruned in format: "module_name,module_name"
  
#### Pruning profiles
- **default** 
  - min-retain-blocks : 0
  - pruning-keep-recent: 400000
  - pruning-keep-every: 100
- **everything** 
  - min-retain-blocks : 0
  - pruning-keep-recent: 10
  - pruning-keep-every: 0
- **nothing** 
  - min-retain-blocks : 0
  - pruning-keep-recent: 0
  - pruning-keep-every: 1
- **emitter** 
  - min-retain-blocks : 300000
  - pruning-keep-recent: 100
  - pruning-keep-every: None
- **rest-light** 
  - min-retain-blocks : 600000
  - pruning-keep-recent: 100000
  - pruning-keep-every: None
- **rest-heavy** 
  - min-retain-blocks : Keep all
  - pruning-keep-recent: 400000
  - pruning-keep-every: 1000
- **peer** 
  - min-retain-blocks : Keep all
  - pruning-keep-recent: 100
  - pruning-keep-every: 30000
- **seed** 
  - min-retain-blocks : 300000
  - pruning-keep-recent: 100
  - pruning-keep-every: None
- **sentry** 
  - min-retain-blocks : 600000
  - pruning-keep-recent: 100
  - pruning-keep-every: None
- **validator** 
  - min-retain-blocks : 600000
  - pruning-keep-recent: 100
  - pruning-keep-every: None

#### Default Module Supported:
- acc
- bank
- staking
- mint
- distribution
- slashing
- gov
- params
- ibc
- upgrade
- evidence
- transfer
- capability

#### Supported App:
- bandchain: Band

#### For Non-supported App:
please provide your chain modules that aren't included in **Default Module Supported** in **--modules** flag
```
