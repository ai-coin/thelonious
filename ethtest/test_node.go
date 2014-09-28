
package ethtest

import (
    "github.com/eris-ltd/eth-go-mods/ethutil"
    "fmt"
    "time"
)

// start the node, start mining, quit
func (t *Test) TestBasic(){
    t.tester("basic", func(eth *EthChain){
        // eth.SetCursor(0) // setting this will invalidate you since this addr isnt in the genesis
        fmt.Println("mining addresS", eth.FetchAddr())
        eth.Start()
        fmt.Println("the node should be running and mining. if not, there are problems. it will stop in 10 seconds ...")
    }, 10)
}

// run a node
func (t* Test) TestRun(){
    t.tester("basic", func(eth *EthChain){
        // eth.SetCursor(0) // setting this will invalidate you since this addr isnt in the genesis
        fmt.Println("mining addresS", eth.FetchAddr())
        eth.Start()
    }, 0)
}

// mine, stop mining, start mining
func (t *Test) TestStopMining(){
    t.tester("mining", func(eth *EthChain){
        fmt.Println("mining addresS", eth.FetchAddr())
        eth.Start()
        time.Sleep(time.Second*10)
        fmt.Println("stopping mining")
        eth.StopMining()
        time.Sleep(time.Second*10)
        fmt.Println("starting mining again")
        eth.StartMining()        
    }, 5)
}

// mine, stop mining, start mining
func (t *Test) TestStopListening(){
    t.tester("mining", func(eth *EthChain){
        eth.Config.Mining = false
        eth.Start()
        time.Sleep(time.Second*1)
        fmt.Println("stopping listening")
        eth.StopListening()
        time.Sleep(time.Second*1)
        fmt.Println("starting listening again")
        eth.StartListening()
    }, 3)
}

func (t *Test) TestRestart(){
    eth := NewEth(nil)
    eth.Config.Mining = true
    eth.Init()
    eth.Start()
    time.Sleep(time.Second*5)
    eth.Stop()
    time.Sleep(time.Second*5)
    eth = NewEth(nil)
    eth.Config.Mining = true
    eth.Init()
    eth.Start()
    time.Sleep(time.Second*5)
}

// note about big nums and values...
func (t *Test) TestBig(){
    a := ethutil.NewValue("100000000000")
    fmt.Println("a, bigint", a, a.BigInt())
    // doesnt work! must do: 
    a = ethutil.NewValue(ethutil.Big("100000000000"))
    fmt.Println("a, bigint", a, a.BigInt())
}

