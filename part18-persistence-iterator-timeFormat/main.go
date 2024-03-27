package main

import (
	"fmt"

	"github.com/easonnong/public-chain/part18-persistence-iterator-timeFormat/BLC"
)

func main() {

	blockchain := BLC.CreateBlockchainWithGenesisBlock()
	defer blockchain.DB.Close()

	blockchain.AddBlockToBlockchain("send 2 bitcoin to satoshi")
	blockchain.AddBlockToBlockchain("send 3 bitcoin to satoshi")
	blockchain.AddBlockToBlockchain("send 4 bitcoin to satoshi")

	fmt.Println()
	blockchain.PrintChain()
}
