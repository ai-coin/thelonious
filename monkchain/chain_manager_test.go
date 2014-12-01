package monkchain

import (
	"container/list"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/eris-ltd/thelonious/monkcrypto"
	"github.com/eris-ltd/thelonious/monkdb"
	"github.com/eris-ltd/thelonious/monkreact"
	"github.com/eris-ltd/thelonious/monkstate"
	"github.com/eris-ltd/thelonious/monkutil"
	"github.com/eris-ltd/thelonious/monkwire"
)

func init() {
	initDB()
}

func initDB() {
	monkutil.ReadConfig(".ethtest", "/tmp/ethtest", "")
	monkutil.Config.Db, _ = monkdb.NewMemDatabase()
}

// So we can generate blocks easily
type fakePow struct{}

func (f fakePow) Search(block *Block, stop chan monkreact.Event) []byte { return nil }
func (f fakePow) Verify(hash []byte, diff *big.Int, nonce []byte) bool  { return true }
func (f fakePow) GetHashrate() int64                                    { return 0 }
func (f fakePow) Turbo(bool)                                            {}

// We need this guy because ProcessWithParent clears txs from the pool
type fakeEth struct{}

func (e *fakeEth) BlockManager() *BlockManager                            { return nil }
func (e *fakeEth) ChainManager() *ChainManager                            { return nil }
func (e *fakeEth) TxPool() *TxPool                                        { return &TxPool{} }
func (e *fakeEth) Broadcast(msgType monkwire.MsgType, data []interface{}) {}
func (e *fakeEth) Reactor() *monkreact.ReactorEngine                      { return monkreact.New() }
func (e *fakeEth) PeerCount() int                                         { return 0 }
func (e *fakeEth) IsMining() bool                                         { return false }
func (e *fakeEth) IsListening() bool                                      { return false }
func (e *fakeEth) Peers() *list.List                                      { return nil }
func (e *fakeEth) KeyManager() *monkcrypto.KeyManager                     { return nil }
func (e *fakeEth) ClientIdentity() monkwire.ClientIdentity                { return nil }
func (e *fakeEth) Db() monkutil.Database                                  { return nil }
func (e *fakeEth) GenesisPointer(block *Block)                            {}
func (e *fakeEth) GenesisModel() GenDougModel                             { return nil }

type fakeDoug struct{}

func (d *fakeDoug) Deploy(block *Block)                                                 {}
func (d *fakeDoug) StartMining(coinbase []byte, parent *Block) bool                     { return false }
func (d *fakeDoug) Difficulty(block, parent *Block) *big.Int                            { return nil }
func (d *fakeDoug) ValidatePerm(addr []byte, role string, state *monkstate.State) error { return nil }
func (d *fakeDoug) ValidateBlock(block *Block, bc *ChainManager) error                  { return nil }
func (d *fakeDoug) ValidateTx(tx *Transaction, state *monkstate.State) error            { return nil }

var (
	FakeEth  = &fakeEth{}
	FakeDoug = &fakeDoug{}
)

func newBlockFromParent(addr []byte, parent *Block) *Block {
	block := CreateBlock(
		parent.state.Trie.Root,
		parent.Hash(),
		addr,
		monkutil.BigPow(2, 32),
		nil,
		"")
	block.MinGasPrice = big.NewInt(10000000000000)
	block.Difficulty = CalcDifficulty(block, parent)
	block.Number = new(big.Int).Add(parent.Number, monkutil.Big1)
	block.GasLimit = block.CalcGasLimit(parent)
	return block
}

// Actually make a block by simulating what miner would do
func makeBlock(bman *BlockManager, parent *Block, i int) *Block {
	addr := monkutil.LeftPadBytes([]byte{byte(i)}, 20)
	block := newBlockFromParent(addr, parent)
	cbase := block.State().GetOrNewStateObject(addr)
	cbase.SetGasPool(block.CalcGasLimit(parent))
	receipts, txs, _, _ := bman.ProcessTransactions(cbase, block.State(), block, block, Transactions{})
	//block.SetTransactions(txs)
	block.SetTxHash(receipts)
	block.SetReceipts(receipts, txs)
	bman.AccumelateRewards(block.State(), block, parent)
	block.State().Update()
	return block
}

// Make a chain with real blocks
// Runs ProcessWithParent to get proper state roots
func makeChain(bman *BlockManager, parent *Block, max int) *BlockChain {
	bman.bc.currentBlock = parent
	bman.bc.currentBlockHash = parent.Hash()
	blocks := make(Blocks, max)
	var td *big.Int
	for i := 0; i < max; i++ {
		block := makeBlock(bman, parent, i)
		// add the parent and its difficulty to the working chain
		// so ProcessWithParent can access it
		bman.bc.workingChain = NewChain(Blocks{parent})
		bman.bc.workingChain.Back().Value.(*link).td = td
		td, _ = bman.ProcessWithParent(block, parent)
		blocks[i] = block
		parent = block
	}
	lchain := NewChain(blocks)
	return lchain
}

// Make a new canonical chain by running TestChain and InsertChain
// on result of makeChain
func newCanonical(n int) (*BlockManager, error) {
	bman := &BlockManager{bc: NewChainManager(FakeDoug), Pow: fakePow{}, th: FakeEth}
	bman.bc.SetProcessor(bman)
	parent := bman.bc.CurrentBlock()
	lchain := makeChain(bman, parent, n)

	_, err := bman.bc.TestChain(lchain)
	if err != nil {
		return nil, err
	}
	bman.bc.InsertChain(lchain)
	return bman, nil
}

// Create a new chain manager starting from given block
// Effectively a fork factory
func newChainManager(block *Block, protocol GenDougModel) *ChainManager {
	bc := &ChainManager{}
	bc.protocol = protocol
	bc.genesisBlock = NewBlockFromBytes(monkutil.Encode(Genesis))
	genDoug = bc.protocol
	if block == nil {
		bc.Reset()
        bc.TD = monkutil.Big("0")
	} else {
		bc.currentBlock = block
		bc.SetTotalDifficulty(monkutil.Big("0"))
        bc.TD = block.BlockInfo().TD
	}
	return bc
}

// Test fork of length N starting from block i
func testFork(t *testing.T, bman *BlockManager, i, N int, f func(td1, td2 *big.Int)) {
	var b *Block = nil
	if i > 0 {
		b = bman.bc.GetBlockByNumber(uint64(i))
	}
	bman2 := &BlockManager{bc: newChainManager(b, FakeDoug), Pow: fakePow{}, th: &fakeEth{}}
	bman2.bc.SetProcessor(bman2)
	parent := bman2.bc.CurrentBlock()
	chainB := makeChain(bman2, parent, N)
	// test second chain against first
	td2, err := bman.bc.TestChain(chainB)
	if err != nil && !IsTDError(err) {
		t.Error("expected chainB not to give errors:", err)
	}
	// Compare difficulties
	f(bman.bc.TD, td2)
}

func TestExtendCanonical(t *testing.T) {
	initDB()
	// make first chain starting from genesis
	bman, err := newCanonical(5)
	if err != nil {
		t.Fatal("Could not make new canonical chain:", err)
	}

	f := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) <= 0 {
			t.Error("expected chainB to have higher difficulty. Got", td2, "expected more than", td1)
		}
	}

	// Start fork from current height (5)
	testFork(t, bman, 5, 1, f)
	testFork(t, bman, 5, 2, f)
	testFork(t, bman, 5, 5, f)
	testFork(t, bman, 5, 10, f)

}

func TestShorterFork(t *testing.T) {
	initDB()
	// make first chain starting from genesis
	bman, err := newCanonical(10)
	if err != nil {
		t.Fatal("Could not make new canonical chain:", err)
	}

	f := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) >= 0 {
			t.Error("expected chainB to have lower difficulty. Got", td2, "expected less than", td1)
		}
	}

	// Sum of numbers must be less than 10
	// for this to be a shorter fork
	testFork(t, bman, 0, 3, f)
	testFork(t, bman, 0, 7, f)
	testFork(t, bman, 1, 3, f)
	testFork(t, bman, 1, 7, f)
	testFork(t, bman, 5, 3, f)
	testFork(t, bman, 5, 4, f)

}

func TestLongerFork(t *testing.T) {
	initDB()
	// make first chain starting from genesis
	bman, err := newCanonical(10)
	if err != nil {
		t.Fatal("Could not make new canonical chain:", err)
	}

	f := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) <= 0 {
			t.Error("expected chainB to have higher difficulty. Got", td2, "expected more than", td1)
		}
	}

	// Sum of numbers must be greater than 10
	// for this to be a longer fork
	testFork(t, bman, 0, 11, f)
	testFork(t, bman, 0, 15, f)
	testFork(t, bman, 1, 10, f)
	testFork(t, bman, 1, 12, f)
	testFork(t, bman, 5, 6, f)
	testFork(t, bman, 5, 8, f)
}

func TestEqualFork(t *testing.T) {
	initDB()
	bman, err := newCanonical(10)
	if err != nil {
		t.Fatal("Could not make new canonical chain:", err)
	}

	f := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) != 0 {
			t.Error("expected chainB to have equal difficulty. Got", td2, "expected less than", td1)
		}
	}

	// Sum of numbers must be equal to 10
	// for this to be an equal fork
	testFork(t, bman, 0, 10, f)
	testFork(t, bman, 1, 9, f)
	testFork(t, bman, 2, 8, f)
	testFork(t, bman, 5, 5, f)
	testFork(t, bman, 6, 4, f)
	testFork(t, bman, 9, 1, f)
}

func TestBrokenChain(t *testing.T) {
	initDB()
	bman, err := newCanonical(10)
	if err != nil {
		t.Fatal("Could not make new canonical chain:", err)
	}

	bman2 := &BlockManager{bc: NewChainManager(FakeDoug), Pow: fakePow{}, th: FakeEth}
	bman2.bc.SetProcessor(bman2)
	parent := bman2.bc.CurrentBlock()

	chainB := makeChain(bman2, parent, 5)
	chainB.Remove(chainB.Front())

	_, err = bman.bc.TestChain(chainB)
	if err == nil {
		t.Error("expected broken chain to return error")
	}
}

func BenchmarkChainTesting(b *testing.B) {
	initDB()
	const chainlen = 1000

	bman, err := newCanonical(5)
	if err != nil {
		b.Fatal("Could not make new canonical chain:", err)
	}

	bman2 := &BlockManager{bc: NewChainManager(FakeDoug), Pow: fakePow{}, th: FakeEth}
	bman2.bc.SetProcessor(bman2)
	parent := bman2.bc.CurrentBlock()

	chain := makeChain(bman2, parent, chainlen)

	stime := time.Now()
	bman.bc.TestChain(chain)
	fmt.Println(chainlen, "took", time.Since(stime))
}
