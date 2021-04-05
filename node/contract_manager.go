package node

import(
	_"fmt"
	"sync"
	"time"

	_"github.com/pkg/errors"
	_"github.com/libp2p/go-libp2p-core/peer"
	"go.uber.org/zap"
	"strconv"

	genapi "omnidisk/gen/api"
	"omnidisk/entities"
	"omnidisk/events"
	"omnidisk/ipfs"
	"omnidisk/messages"
)

type ContractManager struct{
	logger *zap.Logger
	node Node

	connector *Connector

	lock sync.RWMutex
}


func NewContractManager(logger *zap.Logger, node Node, c *Connector) *ContractManager{
	if logger == nil{
		logger = zap.NewNop()
	}

	cm := &ContractManager{
		logger:		logger,
		node:			node,
		connector:	c,
	}

	return cm
}

func (cm *ContractManager) PrepareContract(filepath string, expires int64, cardinality, datatype uint32) (entities.Contract, error){

	var cid string
	var size uint32
	var err error
	if datatype == 1 || datatype == 2{ //encryption passphrase is node's private key
		pk, err := cm.node.getPrivKey()
		if err != nil{
			cm.logger.Error("failed to obtain private key; cannot encrypt!", zap.Error(err))
			return entities.Contract{}, err
		}
		cid, size, err = ipfs.StoreToMFS(datatype, filepath, pk, cm.logger)
	} else{ //no encryption
		cid, size, err = ipfs.StoreToMFS(datatype, filepath, "", cm.logger)
	}
	if err != nil{
		return entities.Contract{}, err
	}

	expiresGo := time.Unix(expires, 0)

	contract := entities.Contract{
		OwnerID:			cm.node.ID().Pretty(),
		Size:				size,
		Cid:				cid,
		Expires:			expiresGo,
		Cardinality:	cardinality,
		Datatype:		datatype,
	}

	return contract, nil
}


//0 means declined, 1 means accepted, 2 means waiting
func getSlot(acks *[]int) int{
	for{
		allDone := true
		for i:=0; i<len(*acks); i++{
			if (*acks)[i] != 1{
				allDone = false
			}
			if (*acks)[i] == 0{
				(*acks)[i] = 2
				return i
			}
		}
		if allDone{
			return -1
		}
		//if this point is reached, means some slots are waiting for ack/decline (timeout)
		time.Sleep(1*time.Second)
	}
}


//get multiaddrs from sub and send them via channel c. Make sure no duplicates are sent
func getCandidates(logger *zap.Logger, sub events.Subscriber, c chan string, doneC chan struct{}, candType string){

	triedAddrs :=  make(map[string] bool)
	for{
		evt, err := sub.Next()
		var addr string
		if err != nil{
			logger.Warn("failed receiving event", zap.Error(err))
			continue
		} else if evt.MarshalToProtobuf().Type == genapi.Event_OFFER_PUB && candType == "offer"{
			addr = evt.MarshalToProtobuf().OfferPub.Offer.Multiaddr
			_, tried := triedAddrs[addr]
			if tried{
				continue
			}
		} else if evt.MarshalToProtobuf().Type == genapi.Event_CONTRACT_PUB && candType != "offer"{
			addr = evt.MarshalToProtobuf().ContractPub.Multiaddr //get addr
			_, tried := triedAddrs[addr] //see it wasn't tried before
			if tried{
				continue
			}
			isCandidate := false
			for _, contract := range evt.MarshalToProtobuf().ContractPub.Contracts{
				if contract.OwnerId == candType{ //holds contract with right owner_id
					isCandidate = true
					break
				}
			}
			if !isCandidate{ //doesn't hold right contract
				triedAddrs[addr] = true //make sure it won't be checked again
				continue
			}
		} else{
			continue
		}

		select{
			case c <- addr:
				triedAddrs[addr] = true
			case <- doneC:
				close(c)
				return
		}
	}
}

//use sub to get addresses of offering nodes
//send them the contract via connector
func (cm *ContractManager) SeekStorage(contract entities.Contract, sub events.Subscriber) (string, error){

	//construct storage request message
	request := messages.StorageRequest{
		Multiaddr:	cm.node.Multiaddr(),
		Contract:	contract,
	}
	var response messages.StorageResponse

	cardinality := contract.Cardinality
	//0 means declined, 1 means accepted, 2 means waiting
	acks := make([]int, cardinality)

	c := make(chan string) //for multiaddr of offering nodes
	doneC := make(chan struct{})
	go getCandidates(cm.logger, sub, c, doneC, "offer")

	for{
		slot := getSlot(&acks)
		if slot == -1{ //DONE
			close(doneC)
			go ipfs.IpfsUnpinAll(contract.Cid, cm.logger)
			return "https://ipfs.io/ipfs/" + contract.Cid, nil
		}

		addr := <-c
		cm.logger.Debug("", zap.String("seeking storage from addr", addr))

		go cm.connector.Send(addr, request, &acks[slot], &response)
	}

	return "", nil
}

func (cm *ContractManager) RetrieveDataV2(datatype uint32, cid string, deindex bool, sub events.Subscriber) ([]string, error){

	var cids []string
	var datatypes []uint32

	if datatype != 666 && cid != ""{
		cm.logger.Info("searching for specific content..")
		cids = append(cids, cid)

		if datatype == 1 || datatype == 2{ //encryption passphrase is node's private key
			pk, err := cm.node.getPrivKey()
			if err != nil{
				cm.logger.Error("failed to obtain private key; cannot decrypt!", zap.Error(err))
				return cids, err
			}
			ipfs.Subdei(datatype, cid, pk, deindex, cm.logger)
		} else{ //no encryption
			ipfs.Subdei(datatype, cid, "", deindex, cm.logger)
		}
		return cids, nil
	}

	timeInit := time.Now()
	triedAddrs :=  make(map[string] bool)

	cm.logger.Info("listening for pubs..")
	for{ //spend 1 min waiting for all contract pubs with my ID
		if timeInit.Add(33 * time.Second).Before(time.Now()){
			cm.logger.Info("listening for pubs: DONE")
			break
		}

		evt, err := sub.Next()
		var addr string
		if err != nil{
			cm.logger.Warn("failed receiving event", zap.Error(err))
			continue
		} else if evt.MarshalToProtobuf().Type != genapi.Event_CONTRACT_PUB{
			continue
		}

		addr = evt.MarshalToProtobuf().ContractPub.Multiaddr //get addr
		_, tried := triedAddrs[addr] //see it wasn't tried before
		if tried{
			continue
		}
		triedAddrs[addr] = true //make sure it won't be checked again

		for _, contract := range evt.MarshalToProtobuf().ContractPub.Contracts{
			if contract.OwnerId == cm.node.ID().Pretty() { //contract with right owner_id
				if cid != "" && contract.Cid != cid{ //searching just for this cid
					continue
				}
				cm.logger.Info("contract found!")
				cids = append(cids, contract.Cid)
				datatypes = append(datatypes, contract.Datatype)
			}
		}
	}

	if len(cids) == 0{
		cm.logger.Info("no pub about stored data was heard.")
	}
	for ix, _ := range cids{
		if datatypes[ix] == 1 || datatypes[ix] == 2{ //encryption passphrase is node's private key
			pk, err := cm.node.getPrivKey()
			if err != nil{
				cm.logger.Error("failed to obtain private key; cannot decrypt!", zap.Error(err))
				return cids, err
			}
			ipfs.Dei(datatypes[ix], cids[ix], pk, deindex, cm.logger)
		} else{ //no encryption
			ipfs.Dei(datatypes[ix], cids[ix], "", deindex, cm.logger)
		}
	}

	return cids, nil
}

func (cm *ContractManager) RetrieveData(request messages.RetrievalRequest, sub events.Subscriber) ([]string, error){
	acks := make([]int, 1)
	response := messages.RetrievalResponse{}

	c := make(chan string) //for multiaddr of nodes holding right contract
	doneC := make(chan struct{})
	go getCandidates(cm.logger, sub, c, doneC, request.OwnerID)

	for{
		slot := getSlot(&acks)
		if slot == -1{ //DONE
			close(doneC)
			return response.IpfsLinks, nil
		}

		addr := <-c
		cm.logger.Debug("requesting data", zap.String("addr", addr))

		go cm.connector.Send(addr, request, &acks[slot], &response)
	}
}

func (cm *ContractManager) PrepareRetrievalRequest(ownerID string) (messages.RetrievalRequest, error){
	request := messages.RetrievalRequest{
		Multiaddr:	cm.node.Multiaddr(),
		OwnerID:		ownerID,
		Timestamp:	time.Now(),
	}

	str := request.Multiaddr + request.OwnerID + strconv.FormatInt(request.Timestamp.Unix(), 10)
	signature, err := cm.node.sign(ownerID, []byte(str))
	if err != nil{
		cm.logger.Error("couldn't sign retrieval request", zap.Error(err))
		return request, err
	}

	request.Signature = string(signature)
	return request, nil
}

