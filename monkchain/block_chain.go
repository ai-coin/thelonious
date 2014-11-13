package monkchain

import (
	"bytes"
	"fmt"
	"math/big"
	"github.com/eris-ltd/thelonious/monklog"
	"github.com/eris-ltd/thelonious/monkutil"
	"github.com/eris-ltd/thelonious/monkcrypto"
)

var chainlogger = monklog.NewLogger("CHAIN")

var DougDifficulty = monkutil.BigPow(2, 17)  // for mining speed

type BlockChain struct {
	Ethereum EthManager
	// The famous, the fabulous Mister GENESIIIIIIS (block)
	genesisBlock *Block
	// Last known total difficulty
	TD *big.Int

	LastBlockNumber uint64

	CurrentBlock  *Block
	LastBlockHash []byte
}

func NewBlockChain(ethereum EthManager) *BlockChain {
	bc := &BlockChain{}
	bc.genesisBlock = NewBlockFromBytes(monkutil.Encode(Genesis))
	bc.Ethereum = ethereum

	bc.setLastBlock()

	return bc
}

func (bc *BlockChain) Genesis() *Block {
	return bc.genesisBlock
}

func (bc *BlockChain) NewBlock(coinbase []byte) *Block {
	var root interface{}
	var lastBlockTime int64
	hash := ZeroHash256

	if bc.CurrentBlock != nil {
		root = bc.CurrentBlock.state.Trie.Root
		hash = bc.LastBlockHash
		lastBlockTime = bc.CurrentBlock.Time
	}

	block := CreateBlock(
		root,
		hash,
		coinbase,
//		monkutil.BigPow(2, 32),
		monkutil.BigPow(2, 12),
		nil,
		"")

	block.MinGasPrice = big.NewInt(10000000000000)

	parent := bc.CurrentBlock
	if parent != nil {
		diff := new(big.Int)

		adjust := new(big.Int).Rsh(parent.Difficulty, 10)
		if block.Time >= lastBlockTime+5 {
			diff.Sub(parent.Difficulty, adjust)
		} else {
			diff.Add(parent.Difficulty, adjust)
		}
		block.Difficulty = DougDifficulty //monkutil.BigPow(2, 12) //diff
		block.Number = new(big.Int).Add(bc.CurrentBlock.Number, monkutil.Big1)
		block.GasLimit = monkutil.BigPow(10, 50) //block.CalcGasLimit(bc.CurrentBlock)

	}

	return block
}

func (bc *BlockChain) HasBlock(hash []byte) bool {
	data, _ := monkutil.Config.Db.Get(hash)
	return len(data) != 0
}

// TODO: At one point we might want to save a block by prevHash in the db to optimise this...
func (bc *BlockChain) HasBlockWithPrevHash(hash []byte) bool {
	block := bc.CurrentBlock

	for ; block != nil; block = bc.GetBlock(block.PrevHash) {
		if bytes.Compare(hash, block.PrevHash) == 0 {
			return true
		}
	}
	return false
}

func (bc *BlockChain) CalculateBlockTD(block *Block) *big.Int {
	blockDiff := new(big.Int)

	for _, uncle := range block.Uncles {
		blockDiff = blockDiff.Add(blockDiff, uncle.Difficulty)
	}
	blockDiff = blockDiff.Add(blockDiff, block.Difficulty)

	return blockDiff
}

func (bc *BlockChain) GenesisBlock() *Block {
	return bc.genesisBlock
}

func (self *BlockChain) GetChainHashesFromHash(hash []byte, max uint64) (chain [][]byte) {
	block := self.GetBlock(hash)
	if block == nil {
		return
	}

	// XXX Could be optimised by using a different database which only holds hashes (i.e., linked list)
	for i := uint64(0); i < max; i++ {
		chain = append(chain, block.Hash())

		if block.Number.Cmp(monkutil.Big0) <= 0 {
			break
		}

		block = self.GetBlock(block.PrevHash)
	}

	return
}




/*
func AddTestNetFunds(block *Block) {
	for _, addr := range []string{
		"51ba59315b3a95761d0863b05ccc7a7f54703d99",
		"e4157b34ea9615cfbde6b4fda419828124b70c78",
		"b9c015918bdaba24b4ff057a92a3873d6eb201be",
		"6c386a4b26f73c802f34673f7248bb118f97424a",
		"cd2a3d9f938e13cd947ec05abc7fe734df8dd826",
		"2ef47100e0787b915105fd5e3f4ff6752079d5cb",
		"e6716f9544a56c530d868e4bfbacb172315bdead",
		"1a26338f0d905e295fccb71fa9ea849ffa12aaf4",
	} {
		codedAddr := monkutil.Hex2Bytes(addr)
		account := block.state.GetAccount(codedAddr)
		account.Balance = monkutil.Big("1606938044258990275541962092341162602522202993782792835301376") //monkutil.BigPow(2, 200)
		block.state.UpdateStateObject(account)
	}
}
*/

func (bc *BlockChain) Fuck(thing string){
    fmt.Println(thing)
    addr := monkcrypto.Sha3Bin([]byte("the genesis doug"))
    block := bc.CurrentBlock
    st := block.state
    c := st.GetAccount(addr)
    fmt.Println("bal:", c.Balance)
    store := c.State.Trie
    it := store.NewIterator()
    it.Each(func(key string, value *monkutil.Value) {
        fmt.Println(monkutil.Bytes2Hex([]byte(key)), value)
    })

}

func (bc *BlockChain) setLastBlock() {

    // check for last block. if none exists, fire up a genesis
	data, _ := monkutil.Config.Db.Get([]byte("LastBlock"))
	if len(data) != 0 {
		block := NewBlockFromBytes(data)
		bc.CurrentBlock = block
		bc.LastBlockHash = block.Hash()
		bc.LastBlockNumber = block.Number.Uint64()

	} else {
        // Prepare the genesis block!
        bc.Ethereum.GenesisPointer(bc.genesisBlock)
        /*
            The less enlightened might do something naiive like:
            -> AddTestNetFunds(bc.genesisBlock, bc.Ethereum)
            But this is DougLand baby!
        */
		bc.Add(bc.genesisBlock)
		fk := append([]byte("bloom"), bc.genesisBlock.Hash()...)
		bc.Ethereum.Db().Put(fk, make([]byte, 255))
		bc.CurrentBlock = bc.genesisBlock
	}
    // set the genDoug model for determining chain permissions
    genDoug = bc.Ethereum.GenesisModel()

	// Set the last know difficulty (might be 0x0 as initial value, Genesis)
	bc.TD = monkutil.BigD(monkutil.Config.Db.LastKnownTD())

	chainlogger.Infof("Last block (#%d) %x\n", bc.LastBlockNumber, bc.CurrentBlock.Hash())

}

func (bc *BlockChain) SetTotalDifficulty(td *big.Int) {
	monkutil.Config.Db.Put([]byte("LTD"), td.Bytes())
	bc.TD = td
}

// Add a block to the chain and record addition information
func (bc *BlockChain) Add(block *Block) {
	bc.writeBlockInfo(block)
	// Prepare the genesis block

	bc.CurrentBlock = block
	bc.LastBlockHash = block.Hash()

	encodedBlock := block.RlpEncode()
	monkutil.Config.Db.Put(block.Hash(), encodedBlock)
	monkutil.Config.Db.Put([]byte("LastBlock"), encodedBlock)
}

func (self *BlockChain) CalcTotalDiff(block *Block) (*big.Int, error) {
	parent := self.GetBlock(block.PrevHash)
	if parent == nil {
		return nil, fmt.Errorf("Unable to calculate total diff without known parent %x", block.PrevHash)
	}

	parentTd := parent.BlockInfo().TD

	uncleDiff := new(big.Int)
	for _, uncle := range block.Uncles {
		uncleDiff = uncleDiff.Add(uncleDiff, uncle.Difficulty)
	}

	td := new(big.Int)
	td = td.Add(parentTd, uncleDiff)
	td = td.Add(td, block.Difficulty)

	return td, nil
}

func (bc *BlockChain) GetBlock(hash []byte) *Block {
	data, _ := monkutil.Config.Db.Get(hash)
	if len(data) == 0 {
		return nil
	}

	return NewBlockFromBytes(data)
}

func (self *BlockChain) GetBlockByNumber(num uint64) *Block {
	block := self.CurrentBlock
	for ; block != nil; block = self.GetBlock(block.PrevHash) {
		if block.Number.Uint64() == num {
			break
		}
	}

	if block != nil && block.Number.Uint64() == 0 && num != 0 {
		return nil
	}

	return block
}

func (self *BlockChain) GetBlockBack(num uint64) *Block {
	block := self.CurrentBlock

	for ; num != 0 && block != nil; num-- {
		block = self.GetBlock(block.PrevHash)
	}

	return block
}

func (bc *BlockChain) BlockInfoByHash(hash []byte) BlockInfo {
	bi := BlockInfo{}
	data, _ := monkutil.Config.Db.Get(append(hash, []byte("Info")...))
	bi.RlpDecode(data)

	return bi
}

func (bc *BlockChain) BlockInfo(block *Block) BlockInfo {
	bi := BlockInfo{}
	data, _ := monkutil.Config.Db.Get(append(block.Hash(), []byte("Info")...))
	bi.RlpDecode(data)

	return bi
}

// Unexported method for writing extra non-essential block info to the db
func (bc *BlockChain) writeBlockInfo(block *Block) {
    if block.Number.Cmp(big.NewInt(0)) != 0{
	    bc.LastBlockNumber++
    }
	bi := BlockInfo{Number: bc.LastBlockNumber, Hash: block.Hash(), Parent: block.PrevHash, TD: bc.TD}

	// For now we use the block hash with the words "info" appended as key
	monkutil.Config.Db.Put(append(block.Hash(), []byte("Info")...), bi.RlpEncode())
}

func (bc *BlockChain) Stop() {
	if bc.CurrentBlock != nil {
		chainlogger.Infoln("Stopped")
	}
}
