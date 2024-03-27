package BLC

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/boltdb/bolt"
)

// database name
const dbName = "blockchain.db"

//table name
const blockTableName = "blocks"

type BlockChain struct {
	// the  latest block hash
	Tip []byte
	// blockchain database
	DB *bolt.DB
}

// add new block to blockchain
func (blockchain *BlockChain) AddBlockToBlockchain(txs []*Transaction) {

	// update database
	err := blockchain.DB.Update(func(tx *bolt.Tx) error {
		// 1.get table
		bucket := tx.Bucket([]byte(blockTableName))

		// 2.create new block
		if bucket != nil {
			// 3.get latest block
			blockBytes := bucket.Get(blockchain.Tip)
			// deserialize
			block := DeserializateBlock(blockBytes)

			// 4.store new block
			newBlock := NewBlock(txs, block.Height+1, block.Hash)
			err := bucket.Put(newBlock.Hash, newBlock.SerializeBlock())
			if err != nil {
				log.Panic(err)
			}

			// 4.update "l"
			err = bucket.Put([]byte("l"), newBlock.Hash)
			if err != nil {
				log.Panic(err)
			}
			// 5.update Tip
			blockchain.Tip = newBlock.Hash
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// print blockchain database
func (blockchain *BlockChain) PrintChain() {

	// create interator
	blockChainIterator := blockchain.CreateIterator()
	for {
		// get current block
		block := blockChainIterator.NextIterator()

		// print block data
		fmt.Printf("Height: %d\n", block.Height)
		fmt.Printf("PrevBlockHash: %x\n", block.PrevBlockHash)
		fmt.Printf("Timestamp: %s\n", time.Unix(block.Timestamp, 0).Format("2006-01-02 03:04:05 PM"))
		fmt.Printf("Hash: %x\n", block.Hash)
		fmt.Printf("Nonce: %d\n", block.Nonce)
		fmt.Println("Txs:")
		for _, tx := range block.Txs {
			fmt.Printf("tx.TxHash=%x\n", tx.TxHash)
			fmt.Printf("Vins:")
			for _, in := range tx.Vins {
				fmt.Printf("{in.TxHash:%x", in.TxHash)
				fmt.Printf(", in.Vout:%d", in.Vout)
				fmt.Printf(", in.ScriptSig:%s}\n", in.ScriptSig)
			}
			fmt.Printf("Vouts:")
			for _, out := range tx.Vouts {
				fmt.Printf("{out.Value:%d", out.Value)
				fmt.Printf(", out.ScriptPubKey:%s}\n", out.ScriptPubKey)
			}
		}
		fmt.Println()

		// doing cycle
		var hashInt big.Int
		hashInt.SetBytes(block.PrevBlockHash)

		if big.NewInt(0).Cmp(&hashInt) == 0 {
			break
		}

	}
}

// get balance
func (blockchain *BlockChain) GetBalance(address string) int64 {
	utxos := blockchain.unUTXOs(address, []*Transaction{})
	var amount int64
	for _, utxo := range utxos {
		amount += utxo.OutPut.Value
	}
	return amount
}

// mine new block
func (blockchain *BlockChain) MineNewBlock(from, to, amount []string) {
	fmt.Println(from)
	fmt.Println(to)
	fmt.Println(amount)

	//1. get txs
	var txs []*Transaction
	var block *Block

	for index, address := range from {
		value, _ := strconv.Atoi(amount[index])
		tx := NewSimpleTransaction(address, to[index], value, blockchain, txs)
		txs = append(txs, tx)
		fmt.Println(tx)
	}

	blockchain.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockTableName))
		if bucket != nil {
			hash := bucket.Get([]byte("l"))

			blockBytes := bucket.Get(hash)

			block = DeserializateBlock(blockBytes)
		}
		return nil
	})

	//2. add new block
	block = NewBlock(txs, block.Height+1, block.Hash)

	blockchain.DB.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(blockTableName))
		if bucket != nil {
			bucket.Put(block.Hash, block.SerializeBlock())

			bucket.Put([]byte("l"), block.Hash)

			blockchain.Tip = block.Hash
		}
		return nil
	})
}

// find spend transations UTXO
func (blockchain *BlockChain) FindSpendableUTXOs(from string, amount int, txs []*Transaction) (int64, map[string][]int) {
	//1. get all UTXO
	utxos := blockchain.unUTXOs(from, txs)
	spendableUTXO := make(map[string][]int)

	//2. traverse utxos
	var value int64

	for _, utxo := range utxos {
		value += utxo.OutPut.Value

		hash := hex.EncodeToString(utxo.TxHash)
		spendableUTXO[hash] = append(spendableUTXO[hash], utxo.Index)

		if value >= int64(amount) {
			break
		}
	}

	if value < int64(amount) {
		fmt.Printf("%s's fund is not enough\n", from)
		os.Exit(1)
	}

	return value, spendableUTXO
}

// get unspent transations
func (blockchain *BlockChain) unUTXOs(address string, txs []*Transaction) []*UTXO {

	var unUTXOs []*UTXO
	spentTxOutputs := make(map[string][]int)

	for _, tx := range txs {
		// Vouts
	work1:
		for index, out := range tx.Vouts {
			if out.UnLockScriptPubKeyWithAddress(address) {
				//if spentTxOutputs != nil {
				fmt.Println("address:", address)
				fmt.Println("spendTXOutputs:", spentTxOutputs)

				if len(spentTxOutputs) == 0 {
					utxo := &UTXO{
						TxHash: tx.TxHash,
						Index:  index,
						OutPut: out,
					}
					unUTXOs = append(unUTXOs, utxo)
				} else {

					for hash, indexArray := range spentTxOutputs {
						txHashStr := hex.EncodeToString(tx.TxHash)
						if hash == txHashStr {

							var isSpendUTXO bool

							for _, outIndex := range indexArray {
								if index == outIndex {
									isSpendUTXO = true
									continue work1
								}

							}
							if !isSpendUTXO {
								utxo := &UTXO{
									TxHash: tx.TxHash,
									Index:  index,
									OutPut: out,
								}
								unUTXOs = append(unUTXOs, utxo)
							}

						} else {
							utxo := &UTXO{
								TxHash: tx.TxHash,
								Index:  index,
								OutPut: out,
							}
							unUTXOs = append(unUTXOs, utxo)
						}
						//}
					}
				}
			}
		}
	}

	blockIterator := blockchain.CreateIterator()

	for {
		block := blockIterator.NextIterator()
		fmt.Println(block)
		fmt.Println()

		// txHash
		for i := len(block.Txs) - 1; i >= 0; i-- {

			tx := block.Txs[i]

			// Vins
			if !tx.IsCoinbaseTransaction() {
				for _, in := range tx.Vins {
					// judge if unlock
					if in.UnLockWithAddress(address) {
						key := hex.EncodeToString(in.TxHash)
						spentTxOutputs[key] = append(spentTxOutputs[key], in.Vout)
					}
				}
			}
			// Vouts
		work2:
			for index, out := range tx.Vouts {
				if out.UnLockScriptPubKeyWithAddress(address) {
					//if spentTxOutputs != nil {
					if len(spentTxOutputs) != 0 {

						var isSpendUTXO bool

						for txHash, indexArray := range spentTxOutputs {

							for _, i := range indexArray {
								if index == i && txHash == hex.EncodeToString(tx.TxHash) {
									isSpendUTXO = true
									continue work2
								}
							}
						}
						if !isSpendUTXO {
							utxo := &UTXO{
								TxHash: tx.TxHash,
								Index:  index,
								OutPut: out,
							}
							unUTXOs = append(unUTXOs, utxo)
						}

					} else {
						utxo := &UTXO{
							TxHash: tx.TxHash,
							Index:  index,
							OutPut: out,
						}
						unUTXOs = append(unUTXOs, utxo)
					}
					//}
				}
			}
		}

		var hashInt big.Int
		hashInt.SetBytes(block.PrevBlockHash)

		if hashInt.Cmp(big.NewInt(0)) == 0 {
			break
		}
	}

	return unUTXOs
}

// get blockchain object
func GetBlockchainObject() *BlockChain {

	// open database
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	var tip []byte

	err = db.View(func(tx *bolt.Tx) error {

		bucket := tx.Bucket([]byte(blockTableName))

		if bucket != nil {
			tip = bucket.Get([]byte("l"))
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return &BlockChain{
		Tip: tip,
		DB:  db,
	}
}

// 1. create genesis blockchain
func CreateBlockchainWithGenesisBlock(address string) {

	// is database exist
	if isDBExist() {
		fmt.Println("genesis block had exist.")
		os.Exit(1)
	}

	// create or open database
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}

	// updata blockchain
	err = db.Update(func(tx *bolt.Tx) error {

		// 1.create table
		bucket, err := tx.CreateBucket([]byte(blockTableName))
		if err != nil {
			log.Panic(err)
		}

		if bucket != nil {
			// create a coinbase transaction
			txCoinbase := NewCoinbaseTransaction(address)

			// 2.create genesis block
			genesisBlock := CreateGenesisBlock([]*Transaction{txCoinbase})

			// 3.store genesis block to table
			err := bucket.Put(genesisBlock.Hash, genesisBlock.SerializeBlock())
			if err != nil {
				log.Panic(err)
			}

			// 4.store latest block hash
			err = bucket.Put([]byte("l"), genesisBlock.Hash)
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

}

// judge isExist database
func isDBExist() bool {
	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		return false
	}
	return true
}
