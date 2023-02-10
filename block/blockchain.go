package block

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goblockchain/utils"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	MiningDifficulty         = 3
	MiningSender             = "THE BLOCKCHAIN"
	MiningReward             = 1.0
	MiningTimeSec            = 20
	BlockchainPortRangeStart = 5000
	BlockchainPortRangeEnd   = 5003
	NeighborIpRangeStart     = 0
	NeighborIpRangeEnd       = 1
	ChainNeighborSyncTimeSec = 20
)

type Block struct {
	timestamp    int64
	nonce        int
	previousHash [32]byte
	transactions []*Transaction
}

func NewBlock(nonce int, previousHash [32]byte, transactions []*Transaction) *Block {
	return &Block{
		timestamp:    time.Now().UnixNano(),
		nonce:        nonce,
		previousHash: previousHash,
		transactions: transactions,
	}
}
func (b *Block) Print() {
	fmt.Printf("timestamp     	%d\n", b.timestamp)
	fmt.Printf("nonce         	%d\n", b.nonce)
	fmt.Printf("previous_hash 	%x\n", b.previousHash)
	for _, t := range b.transactions {
		t.Print()
	}
}
func (b *Block) Hash() [32]byte {
	m, _ := json.Marshal(b)
	return sha256.Sum256([]byte(m))
}
func (b *Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Timestamp    int64          `json:"timestamp"`
		Nonce        int            `json:"nonce"`
		PreviousHash string         `json:"previous-hash"`
		Transactions []*Transaction `json:"transactions"`
	}{
		Timestamp:    b.timestamp,
		Nonce:        b.nonce,
		PreviousHash: fmt.Sprintf("%x", b.previousHash),
		Transactions: b.transactions,
	})
}

type Blockchain struct {
	transactionPool   []*Transaction
	chain             []*Block
	blockchainAddress string
	port              uint16
	mux               sync.Mutex
	neighbors         []string
	muxNeighbors      sync.Mutex
}

func NewBlockchain(blockchainAddress string, port uint16) *Blockchain {
	b := &Block{}
	bc := new(Blockchain)
	bc.blockchainAddress = blockchainAddress
	bc.CreateBlock(0, b.Hash())
	bc.port = port
	return bc
}
func (bc *Blockchain) Chain() []*Block {
	return bc.chain
}
func (bc *Blockchain) Run() {
	bc.StartSyncNeighbors()
}
func (bc *Blockchain) SetNeighbors() {
	bc.neighbors = utils.FindNeighbors(utils.GetHost(), bc.port, NeighborIpRangeStart, NeighborIpRangeEnd, BlockchainPortRangeStart, BlockchainPortRangeEnd)
	log.Printf("%v", bc.neighbors)
}
func (bc *Blockchain) SyncNeighbors() {
	bc.muxNeighbors.Lock()
	defer bc.muxNeighbors.Unlock()
	bc.SetNeighbors()
}
func (bc *Blockchain) StartSyncNeighbors() {
	bc.SyncNeighbors()
	_ = time.AfterFunc(time.Second*ChainNeighborSyncTimeSec, bc.StartSyncNeighbors)
}
func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Blocks []*Block `json:"chains"`
	}{
		Blocks: bc.chain,
	})
}
func (b *Block) PreviousHash() [32]byte {
	return b.previousHash
}
func (b *Block) Transactions() []*Transaction {
	return b.transactions
}
func (b *Block) Nonce() int {
	return b.nonce
}
func (bc *Blockchain) CreateBlock(nonce int, previousHash [32]byte) *Block {
	b := NewBlock(nonce, previousHash, bc.transactionPool)
	bc.chain = append(bc.chain, b)
	bc.transactionPool = []*Transaction{}
	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("https://%s/transaction", n)
		client := &http.Client{}
		req, _ := http.NewRequest("DELETE", endpoint, nil)
		resp, _ := client.Do(req)
		log.Printf("%v", resp)
	}
	return b
}
func (b *Block) UnmarshalJSON(data []byte) error {
	var previousHash string
	v := &struct {
		Timestamp    *int64          `json:"timestamp"`
		Nonce        *int            `json:"nonce"`
		PreviousHash *string         `json:"previous_hash"`
		Transaction  *[]*Transaction `json:"transactions"`
	}{
		Timestamp:    &b.timestamp,
		Nonce:        &b.nonce,
		PreviousHash: &previousHash,
		Transaction:  &b.transactions,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	ph, _ := hex.DecodeString(*v.PreviousHash)
	copy(b.previousHash[:], ph[:32])
	return nil
}
func (bc *Blockchain) UnmarshalJSON(data []byte) error {
	v := &struct {
		Blocks *[]*Block `json:"chain"`
	}{
		Blocks: &bc.chain,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}
func (bc *Blockchain) TransactionPool() []*Transaction {
	return bc.transactionPool
}
func (bc *Blockchain) ClearTransactionPool() {
	bc.transactionPool = bc.transactionPool[:0]
}
func (bc *Blockchain) LastBlock() *Block {
	return bc.chain[len(bc.chain)-1]
}
func (bc *Blockchain) Print() {
	for i, block := range bc.chain {
		fmt.Printf("%s Chain %d %s\n", strings.Repeat("=", 25), i, strings.Repeat("=", 25))
		block.Print()
	}
	fmt.Printf("%s\n", strings.Repeat("*", 25))
}
func (bc *Blockchain) CreateTransaction(sender string, recipient string, value float32,
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	isTransaction := bc.AddTransaction(sender, recipient, value, senderPublicKey, s)
	if isTransaction {
		for _, n := range bc.neighbors {
			publicKeyStr := fmt.Sprintf("%064x%064x", senderPublicKey.X.Bytes(), senderPublicKey.Y.Bytes())
			signturaStr := s.String()
			bt := &TransactionRequest{
				&sender, &recipient, &publicKeyStr, &value, &signturaStr}
			m, _ := json.Marshal(bt)
			buf := bytes.NewBuffer(m)
			endpoint := fmt.Sprintf("https://%s/transactions", n)
			client := &http.Client{}
			req, _ := http.NewRequest("PUT", endpoint, buf)
			resp, _ := client.Do(req)
			log.Printf("%v", resp)
		}
	}
	return isTransaction
}
func (bc *Blockchain) AddTransaction(sender string, recipient string, value float32,
	senderPublicKey *ecdsa.PublicKey, s *utils.Signature) bool {
	t := NewTransaction(sender, recipient, value)
	if sender == MiningSender {
		bc.transactionPool = append(bc.transactionPool, t)
		return true
	}

	if bc.VerityTransactionSignature(senderPublicKey, s, t) {
		/*if bc.CalculateTotalAmount(sender) < value {
			log.Println("ERROR: Not enough balance in a wallet")
			return false
		}*/
		bc.transactionPool = append(bc.transactionPool, t)
		return true
	} else {
		log.Println("ERROR: Verify Transaction")
	}
	return false
}
func (bc *Blockchain) VerityTransactionSignature(senderPublicKey *ecdsa.PublicKey, s *utils.Signature, t *Transaction) bool {
	m, _ := json.Marshal(t)
	h := sha256.Sum256([]byte(m))
	return ecdsa.Verify(senderPublicKey, h[:], s.R, s.S)
}
func (bc *Blockchain) CopyTransactionPool() []*Transaction {
	transactions := make([]*Transaction, 0)
	for _, t := range bc.transactionPool {
		transactions = append(transactions, NewTransaction(t.senderBlockchainAddress, t.recipientBlockchainAddress, t.value))
	}
	return transactions
}
func (bc *Blockchain) ValidProof(nonce int, previousHash [32]byte, transactions []*Transaction, difficulty int) bool {
	zeros := strings.Repeat("0", difficulty)
	guessBlock := Block{0, nonce, previousHash, transactions}
	guessHashStr := fmt.Sprintf("%x", guessBlock.Hash())
	return guessHashStr[:difficulty] == zeros
}
func (bc *Blockchain) ProofOfWork() int {
	transactions := bc.CopyTransactionPool()
	previousHash := bc.LastBlock().Hash()
	nonce := 0
	for !bc.ValidProof(nonce, previousHash, transactions, MiningDifficulty) {
		nonce += 1
	}
	return nonce
}
func (bc *Blockchain) Mining() bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.TransactionPool()) == 0 {
		return false
	}
	bc.AddTransaction(MiningSender, bc.blockchainAddress, MiningReward, nil, nil)
	nonce := bc.ProofOfWork()
	previousHash := bc.LastBlock().Hash()
	bc.CreateBlock(nonce, previousHash)
	log.Println("action=mining,status=success")
	return true
}
func (bc *Blockchain) StartMining() {
	bc.Mining()
	_ = time.AfterFunc(MiningTimeSec*time.Second, bc.StartMining)
}
func (bc *Blockchain) ResolveConflicts() bool {
	var longestChain []*Block = nil
	maxLength := len(bc.chain)
	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("https://%s/chain", n)
		resp, _ := http.Get(endpoint)
		if resp.StatusCode == 200 {
			var bcResp Blockchain
			decoder := json.NewDecoder(resp.Body)
			_ = decoder.Decode(&bcResp)
			chain := bcResp.chain
			if len(chain) > maxLength && bc.ValidChain(chain) {
				maxLength = len(chain)
				longestChain = chain
			}
		}
	}
	if longestChain != nil {
		bc.chain = longestChain
		log.Printf("Resovle conflicts replaceed")
		return true
	}
	log.Printf("Resovle conflicts not replaced")
	return false
}
func (bc *Blockchain) CalculateTotalAmount(blockchainAddress string) float32 {
	var totalAmount float32 = 0.0
	for _, b := range bc.chain {
		for _, t := range b.transactions {
			value := t.value
			if blockchainAddress == t.recipientBlockchainAddress {
				totalAmount += value
			}
			if blockchainAddress == t.senderBlockchainAddress {
				totalAmount -= value
			}
		}
	}
	return totalAmount
}
func (bc *Blockchain) ValidChain(chain []*Block) bool {
	preBlock := chain[0]
	currentIndex := 1
	for currentIndex < len(chain) {
		b := chain[currentIndex]
		if b.previousHash != preBlock.Hash() {
			return false
		}
		if !bc.ValidProof(b.Nonce(), b.PreviousHash(), b.Transactions(), MiningDifficulty) {
			return false
		}
		preBlock = b
		currentIndex += 1
	}
	return true
}
func (t *Transaction) UnmarshalJSON(data []byte) error {
	v := &struct {
		Sender    *string  `json:"sender_blockchain_address"'`
		Recipient *string  `json:"recipient_blockchain_address"`
		Value     *float32 `json:"value"`
	}{
		Sender:    &t.senderBlockchainAddress,
		Recipient: &t.recipientBlockchainAddress,
		Value:     &t.value,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return nil
}

type Transaction struct {
	senderBlockchainAddress    string
	recipientBlockchainAddress string
	value                      float32
}

func NewTransaction(sender string, recipient string, value float32) *Transaction {
	return &Transaction{
		senderBlockchainAddress:    sender,
		recipientBlockchainAddress: recipient,
		value:                      value,
	}
}
func (t *Transaction) Print() {
	fmt.Printf("%s\n", strings.Repeat("-", 40))
	fmt.Printf("sender_blockchain_address 	%s\n", t.senderBlockchainAddress)
	fmt.Printf("recipient_blockchain_address %s\n", t.recipientBlockchainAddress)
	fmt.Printf("value 						%.1f\n", t.value)
}
func (t *Transaction) MarshaJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sender    string  `json:"sender_blockchain_address"`
		Recipient string  `json:"recipient_blockchain_address"`
		Value     float32 `json:"value"`
	}{
		Sender:    t.senderBlockchainAddress,
		Recipient: t.recipientBlockchainAddress,
		Value:     t.value,
	})
}

type TransactionRequest struct {
	SenderBlockchainAddress    *string  `json:"sender_blockchain_address"`
	RecipientBlockchainAddress *string  `json:"recipient_blockchain_address"`
	SenderPublicKey            *string  `json:"sender_public_key"`
	Value                      *float32 `json:"value"`
	Signature                  *string  `json:"signature"`
}

func (tr *TransactionRequest) Validate() bool {
	if tr.Value == nil ||
		tr.Signature == nil ||
		tr.SenderBlockchainAddress == nil ||
		tr.RecipientBlockchainAddress == nil ||
		tr.SenderPublicKey == nil {
		return false
	}
	return true
}

type AmountResponse struct {
	Amount float32 `json:"amount"`
}

func (ar *AmountResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Amount float32 `json:"amount"`
	}{
		Amount: ar.Amount,
	})
}
