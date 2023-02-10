package main

import (
	"encoding/json"
	"goblockchain/block"
	"goblockchain/utils"
	"goblockchain/wallet"
	"io"
	"log"
	"net/http"
	"strconv"
)

var cache = make(map[string]*block.Blockchain)

type BlockchainServer struct {
	port uint16
}

func NewBlockchainServer(port uint16) *BlockchainServer {
	return &BlockchainServer{port}
}
func (bcs *BlockchainServer) Port() uint16 {
	return bcs.port
}
func (bcs *BlockchainServer) GetBlockchain() *block.Blockchain {
	bc, ok := cache["blockchain"]
	if !ok {
		minersWallet := wallet.NewWallet()
		bc = block.NewBlockchain(minersWallet.BlockchainAddress(), bcs.Port())
		cache["blockchain"] = bc
		log.Printf("private_key %v", minersWallet.PrivateKeyStr())
		log.Printf("public_key %v", minersWallet.PublicKeyStr())
		log.Printf("blockchain_address %v", minersWallet.BlockchainAddress())
	}
	return bc
}
func (bcs *BlockchainServer) GetChain(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		w.Header().Add("Content-Type", "application/json")
		bc := bcs.GetBlockchain()
		m, _ := bc.MarshalJSON()
		io.WriteString(w, string(m[:]))
	default:
		log.Printf("ERROR: Invalid HTTP Method")
	}
}
func (bcs *BlockchainServer) Transactions(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		w.Header().Add("Content-Type", "application/type")
		bc := bcs.GetBlockchain()
		transaction := bc.TransactionPool()
		m, _ := json.Marshal(struct {
			Transaction []*block.Transaction `json:"transaction"`
			length      int                  `json:"length"`
		}{
			transaction,
			len(transaction),
		})
		io.WriteString(w, string(m[:]))
	case http.MethodPost:
		decode := json.NewDecoder(req.Body)
		var t *block.TransactionRequest
		err := decode.Decode(&t)
		if err != nil {
			log.Printf("ERROR: %v", err)
			io.WriteString(w, string(utils.JsonStatus("fail")))
		}
		if !t.Validate() {
			log.Println("ERROR: missing field(s)")
			io.WriteString(w, string(utils.JsonStatus("fail")))
			return
		}
		publicKey := utils.PublicKeyFromString(*t.SenderPublicKey)
		signature := utils.SignatureFromString(*t.Signature)
		bc := bcs.GetBlockchain()
		isCreate := bc.CreateTransaction(*t.SenderBlockchainAddress,
			*t.RecipientBlockchainAddress, *t.Value, publicKey, signature)
		w.Header().Add("Content-Type", "application/type")
		var m []byte
		if !isCreate {
			w.WriteHeader(http.StatusBadRequest)
			m = utils.JsonStatus("fail")
		} else {
			w.WriteHeader(http.StatusCreated)
			m = utils.JsonStatus("success")
		}
		io.WriteString(w, string(m))
	case http.MethodPut:
		decode := json.NewDecoder(req.Body)
		var t *block.TransactionRequest
		err := decode.Decode(&t)
		if err != nil {
			log.Printf("ERROR: %v", err)
			io.WriteString(w, string(utils.JsonStatus("fail")))
		}
		if !t.Validate() {
			log.Println("ERROR: missing field(s)")
			io.WriteString(w, string(utils.JsonStatus("fail")))
			return
		}
		publicKey := utils.PublicKeyFromString(*t.SenderPublicKey)
		signature := utils.SignatureFromString(*t.Signature)
		bc := bcs.GetBlockchain()
		isUpdate := bc.AddTransaction(*t.SenderBlockchainAddress,
			*t.RecipientBlockchainAddress, *t.Value, publicKey, signature)
		w.Header().Add("Content-Type", "application/type")
		var m []byte
		if !isUpdate {
			w.WriteHeader(http.StatusBadRequest)
			m = utils.JsonStatus("fail")
		} else {
			m = utils.JsonStatus("success")
		}
		io.WriteString(w, string(m))
	case http.MethodDelete:
		bc := bcs.GetBlockchain()
		bc.ClearTransactionPool()
		io.WriteString(w, string(utils.JsonStatus("success")))

	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Println("ERROR: Invalid HTTP Method")
	}
}
func (bcs *BlockchainServer) Mine(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		bc := bcs.GetBlockchain()
		isMinde := bc.Mining()
		var m []byte
		if !isMinde {
			w.WriteHeader(http.StatusBadRequest)
			m = utils.JsonStatus("fail")
		} else {
			w.WriteHeader(http.StatusCreated)
			m = utils.JsonStatus("success")
		}
		io.WriteString(w, string(m))
	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Println("ERROR: Invalid HTTP Method")
	}
}
func (bcs *BlockchainServer) StartMine(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		bc := bcs.GetBlockchain()
		bc.StartMining()
		var m []byte
		w.WriteHeader(http.StatusCreated)
		m = utils.JsonStatus("success")
		io.WriteString(w, string(m))
	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Println("ERROR: Invalid HTTP Method")
	}
}
func (bcs *BlockchainServer) Amount(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		blockchainAddress := req.URL.Query().Get("blockchain_address")
		amount := bcs.GetBlockchain().CalculateTotalAmount(blockchainAddress)
		ar := &block.AmountResponse{Amount: amount}
		m, _ := ar.MarshalJSON()
		w.Header().Add("Content-Type", "application/json")
		io.WriteString(w, string(m[:]))
	}
}
func (bcs *BlockchainServer) Run() {
	bcs.GetBlockchain().Run()
	http.HandleFunc("/", bcs.GetChain)
	http.HandleFunc("/transactions", bcs.Transactions)
	http.HandleFunc("/mind", bcs.Mine)
	http.HandleFunc("/mind/start", bcs.StartMine)
	http.HandleFunc("/amount", bcs.Amount)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+strconv.Itoa(int(bcs.Port())), nil))
}
