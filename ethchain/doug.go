package ethchain

import (
    "fmt"
    "os"
    "math/big"
    "path"
    "io/ioutil"
    "github.com/eris-ltd/eth-go-mods/ethutil"    
    "github.com/eris-ltd/eth-go-mods/ethcrypto"    
    "github.com/eris-ltd/eth-go-mods/ethstate"    
    "github.com/eris-ltd/eth-go-mods/ethtrie"    
)

func GenesisPointer(block *Block, eth EthManager){
    //GenesisTxs(block, eth)
    //Valids(block, eth)
    GenesisTxsByDoug(block, eth)
}


var (

    DougDifficulty = ethutil.BigPow(2, 17)

    GENDOUG = ethcrypto.Sha3Bin([]byte("the genesis doug"))[12:] //[]byte("\x00"*16 + "DOUG")
    MINERS = "01"
    TXERS = "02"

    GoPath = os.Getenv("GOPATH")
    ContractPath = path.Join(GoPath, "src", "github.com", "eris-ltd", "eth-go-mods", "ethtest", "contracts")
)

func DougValidate(addr []byte, state *ethstate.State, role string) bool{
    fmt.Println("validating addr for role", role)
    genDoug := state.GetStateObject(GENDOUG)

    var N string
    switch(role){
        case "tx":
            N = TXERS
        case "miner":
            N = MINERS
        default:
            return false
    }

    caddr := genDoug.GetAddr(ethutil.Hex2Bytes(N))
    c := state.GetOrNewStateObject(caddr.Bytes())

    valid := c.GetAddr(addr)

    return !valid.IsNil()
}

// create a new tx from a script, with dummy keypair
func NewGenesisContract(scriptFile string) *Transaction{
    // if mutan, load the script. else, pass file name
    var s string
    if scriptFile[len(scriptFile)-3:] == ".mu"{
        r, err := ioutil.ReadFile(scriptFile)
        if err != nil{
            fmt.Println("could not load contract!", scriptFile, err)
            os.Exit(0)
        }
        s = string(r)
    } else{
        s = scriptFile
    }
    script, err := ethutil.Compile(string(s), false) 
    if err != nil{
        fmt.Println("failed compile", err)
        os.Exit(0)
    }
    fmt.Println("script: ", script)
    // dummy keys for signing
    keys := ethcrypto.GenerateNewKeyPair() 

    // create tx
    tx := NewContractCreationTx(ethutil.Big("543"), ethutil.Big("10000"), ethutil.Big("10000"), script)
    tx.Sign(keys.PrivateKey)

    return tx
}

// apply tx to genesis block
func SimpleTransitionState(addr []byte, block *Block, tx *Transaction) *Receipt{
    state := block.State()
    st := NewStateTransition(ethstate.NewStateObject(block.Coinbase), tx, state, block)
    st.AddGas(ethutil.Big("10000000000000000000000000000000000000000000000000000000000000000000000000000000000")) // gas is silly, but the vm needs it

    fmt.Println("man oh man", ethutil.Bytes2Hex(addr))
    receiver := state.NewStateObject(addr)
    receiver.Balance = ethutil.Big("123456789098765432")
    receiver.InitCode = tx.Data
    receiver.State = ethstate.New(ethtrie.New(ethutil.Config.Db, ""))
    sender := state.GetOrNewStateObject(tx.Sender())  
    value := ethutil.Big("12342")

    msg := state.Manifest().AddMessage(&ethstate.Message{
        To: receiver.Address(), From: sender.Address(),
        Input:  tx.Data,
        Origin: sender.Address(),
        Block:  block.Hash(), Timestamp: block.Time, Coinbase: block.Coinbase, Number: block.Number,
        Value: value,
    })
    code, err := st.Eval(msg, receiver.Init(), receiver, "init")
    if err != nil{
        fmt.Println("Eval error in simple transition state:", err)
        os.Exit(0)
    }
    receiver.Code = code
    msg.Output = code

    receipt := &Receipt{tx, ethutil.CopyBytes(state.Root().([]byte)), new(big.Int)}
    return receipt
}

/*
     Set genesis block functions
*/


// add addresses and a simple contract
func GenesisTxs(block *Block, eth EthManager){
    // private keys for these are stored in keys.txt
	for _, addr := range []string{
        "bbbd0256041f7aed3ce278c56ee61492de96d001",
        "b9398794cafb108622b07d9a01ecbed3857592d5",
	} {
        AddAccount(addr, "1606938044258990275541962092341162602522202993782792835301376", block)
	}

    txs := Transactions{}
    receipts := []*Receipt{}

    addr := ethcrypto.Sha3Bin([]byte("the genesis doug"))
    tx := NewGenesisContract(path.Join(ContractPath, "test.mu"))
    receipt := SimpleTransitionState(addr, block, tx)

    txs = append(txs, tx) 
    receipts = append(receipts, receipt)

    block.SetReceipts(receipts, txs)
    block.State().Update()  
    block.State().Sync()  
}


func AddAccount(addr, balance string, block *Block){
    codedAddr := ethutil.Hex2Bytes(addr)
    account := block.State().GetAccount(codedAddr)
    account.Balance = ethutil.Big(balance) //ethutil.BigPow(2, 200)
    block.State().UpdateStateObject(account)
}

// doug and lists of valid miners/txers
func Valids(block *Block, eth EthManager){
    addrs := []string{
        "bbbd0256041f7aed3ce278c56ee61492de96d001",
        "b9398794cafb108622b07d9a01ecbed3857592d5",
    }
    // private keys for these are stored in keys.txt
	for _, addr := range addrs{
        AddAccount(addr, "1606938044258990275541962092341162602522202993782792835301376", block)
	}
  
    // set up main contract addrs
    doug := ethcrypto.Sha3Bin([]byte("the genesis doug"))[12:]
    txers := ethcrypto.Sha3Bin([]byte("txers"))[12:]
    miners := ethcrypto.Sha3Bin([]byte("miners"))[12:]
    // create accounts
    Doug := block.State().GetOrNewStateObject(doug)
    Txers := block.State().GetOrNewStateObject(txers)
    Miners := block.State().GetOrNewStateObject(miners)
    // add addresses into DOUG
    Doug.SetAddr([]byte("\x00"), doug)
    Doug.SetAddr([]byte("\x01"), txers)
    Doug.SetAddr([]byte("\x02"), miners)
    // add permitted transactors to txers contract 
    for _, a := range addrs{
        Txers.SetAddr(ethutil.Hex2Bytes(a), 1)
    }
    // add permitted miners to miners contract 
    Miners.SetAddr(ethutil.Hex2Bytes(addrs[0]), 1)

    block.State().Update()  
    block.State().Sync()
}

// add addresses and a simple contract
func GenesisTxsByDoug(block *Block, eth EthManager){
    // private keys for these are stored in keys.txt
	for _, addr := range []string{
        "bbbd0256041f7aed3ce278c56ee61492de96d001",
        "b9398794cafb108622b07d9a01ecbed3857592d5",
	} {
        AddAccount(addr, "1606938044258990275541962092341162602522202993782792835301376", block)
	}

    fmt.Println("TXS BY DOUG!!")

    txs := Transactions{}
    receipts := []*Receipt{}

    addr := ethcrypto.Sha3Bin([]byte("the genesis doug"))
    tx := NewGenesisContract(path.Join(ContractPath, "test.lll"))
    fmt.Println(tx.String())
    receipt := SimpleTransitionState(addr, block, tx)

    txs = append(txs, tx) 
    receipts = append(receipts, receipt)

    block.SetReceipts(receipts, txs)
    block.State().Update()  
    block.State().Sync()  
}

