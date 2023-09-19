# Build and start Poolsea node locally

---

> Supported only **Mac** or **Linux** **arm64/amd64** operating system

## Preparation

- ### Install Go lang
  [Installation guide](https://go.dev/doc/install)
- ### Install dependencies 
    Run next command:
    ```shell
    go mod vendor
    ```
  
---

## Build and start

1) #### Build smartnode cli files 
    Go to `rocketpool-cli` folder:  
    ```shell
    cd rocketpool-cli
   ```  
    Run build script:  
    ```shell
    ./build.sh
    ```
    > Script should generate next 4 files:  
   > `rocketpool-cli-darwin-amd64` - cli for macOS amd64  
   > `rocketpool-cli-darwin-arm64` - cli for macOS arm64   
   > `rocketpool-cli-linux-amd64` - cli for Linux amd64  
   > `rocketpool-cli-linux-arm64` - cli for Linux arm64 
   
2) #### Install packages for run smartnode cli
   Based on your operating system, insert instead of <build-cli> the name of the file from the description above and run next script:
    ```shell
    ./<build-cli> service install -d
    ```
   
3) #### Configure your client and start
    Run next command line:
    ```shell
    ./<build-cli> service config
    ```
   > Follow terminal UI instructions, save setting and start your node

---

> **All commands for interact with node you can find in `README.md` file**

---
> **For all next guides make sure that Execution client(ETH1) and Consensus client(ETH2) synced.**

## Register node in protocol
For register node run next command: 
```shell
./<build-cli> node rigister
```

## Create minipool
For create minipool run next command:
```shell
./<build-cli> node deposit
```
*After node attestation delay will past you should move minipool to staking status.*  
Run next command:  
```shell
./<build-cli> minipool stake
```

## Enter node to smoothing pool
>This means that your node will be registered as a validator and the commission from the network will be sent to the smoothing pool contract. This contract is a pool for distributing PLS rewards from there.

Run next command:
```shell
./<build-cli> node join-smoothing-pool
```

## Check node rewards
Run next command:
```shell
./<build-cli> node rewards
```

## Claim node rewards
Run next command:
```shell
./<build-cli> node claim-rewards
```