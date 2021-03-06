package broadcaster

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/tranvictor/ethutils"
)

var SharedBroadcaster *Broadcaster
var once sync.Once

// Broadcaster takes a signed tx and try to broadcast it to all
// nodes that it manages as fast as possible. It returns a map of
// failures and a bool indicating that the tx is broadcasted to
// at least 1 node
type Broadcaster struct {
	clients map[string]*rpc.Client
}

func (self *Broadcaster) GetNodes() map[string]*rpc.Client {
	return self.clients
}

func (self *Broadcaster) broadcast(
	ctx context.Context,
	id string, client *rpc.Client, data string,
	wg *sync.WaitGroup, failures *sync.Map) {
	defer wg.Done()
	err := client.CallContext(ctx, nil, "eth_sendRawTransaction", data)

	if err != nil {
		failures.Store(id, err)
	}
}

func (self *Broadcaster) BroadcastTx(tx *types.Transaction) (string, bool, error) {
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return "", false, makeError(map[string]error{
			"tx": fmt.Errorf("Tx is not valid, couldn't use rlp to encode it"),
		})
	}
	return self.Broadcast(common.ToHex(data))
}

// data must be hex encoded of the signed tx
func (self *Broadcaster) Broadcast(data string) (string, bool, error) {
	failures := sync.Map{}
	wg := sync.WaitGroup{}
	for id, client := range self.clients {
		wg.Add(1)
		timeout, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		self.broadcast(timeout, id, client, data, &wg, &failures)
		defer cancel()
	}
	wg.Wait()
	result := map[string]error{}
	failures.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			log.Printf("Broadcast: key (%v) cannot be asserted to string", key)
			return true
		}
		err, ok := value.(error)
		if !ok {
			log.Printf("Broadcast: value (%v) cannot be asserted to error", value)
			return true
		}
		result[k] = err
		return true
	})
	return ethutils.RawTxToHash(data), len(result) != len(self.clients) && len(self.clients) > 0, makeError(result)
}

func NewRopstenBroadcaster() *Broadcaster {
	nodes := map[string]string{
		"ropsten-infura": "https://ropsten.infura.io",
	}
	clients := map[string]*rpc.Client{}
	for name, c := range nodes {
		client, err := rpc.Dial(c)
		if err != nil {
			log.Printf("Couldn't connect to: %s - %v", c, err)
		} else {
			clients[name] = client
		}
	}
	return &Broadcaster{
		clients: clients,
	}
}

func NewBroadcaster() *Broadcaster {
	once.Do(func() {
		nodes := map[string]string{
			"mainnet-quiknode": "https://optionally-pleasant-horse.quiknode.io/9d72a0f8-0d8b-4e4c-aef1-eb529e05cdb9/V1ZsC_tuomfETYotFo4KKA==/",
			"mainnet-infura":   "https://mainnet.infura.io",
			"mainnet-kyber":    "https://semi-node.kyber.network",
		}
		clients := map[string]*rpc.Client{}
		for name, c := range nodes {
			client, err := rpc.Dial(c)
			if err != nil {
				log.Printf("Couldn't connect to: %s - %v", c, err)
			} else {
				clients[name] = client
			}
		}
		SharedBroadcaster = &Broadcaster{
			clients: clients,
		}
	})
	return SharedBroadcaster
}
