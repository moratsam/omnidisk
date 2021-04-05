package node

import (
	"encoding/json"
	_"fmt"
	"io/ioutil"
	"time"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"omnidisk/entities"
	"omnidisk/ipfs"
	"omnidisk/messages"
)

const(
	contracts_filepath = "/home/o/kias/go/src/omnidisk/"
)


type OfferManager struct{
	logger	*zap.Logger
	node		Node

	contractManager *ContractManager

	//contracts specifying the data this node is saving
	contracts []entities.Contract

	isOfferSet bool
	offer entities.Offer

	lock sync.RWMutex
}

func NewOfferManager(logger *zap.Logger, node Node) *OfferManager{
	if logger == nil{
		logger = zap.NewNop()
	}

	om := &OfferManager{
		logger:	logger,
		node:		node,
		isOfferSet:	false,
		offer:	entities.Offer{},
	}

	om.loadFromDisk()
	go om.contractTrimmer()

	return om
}

func (om *OfferManager) loadFromDisk(){
	//this is used to capture reading from .json and convert Size and Cardinality to uint32
	type contract struct{
		OwnerID		string		`json:"owner_id"`
		Size			int			`json:"size"`
		Cid			string		`json:"cid"`
		Expires		time.Time	`json:"expires"`
		Cardinality	int			`json:"cardinality"`
		Datatype		int			`json:"datatype"`
	}

	filepath := contracts_filepath + om.node.ID().Pretty() + "_contracts.json"
	om.logger.Info("loading contracts from disk", zap.String("filepath", filepath))
	var tmp_contracts []contract
	c, err := ioutil.ReadFile(filepath)
	if err != nil{
		om.logger.Warn("failed reading contracts from disk", zap.Error(err))
		om.contracts = []entities.Contract{}
		return
	}
	if err := json.Unmarshal(c, &tmp_contracts); err != nil{
		om.logger.Warn("failed unmarshalling contracts from file", zap.Error(err))
		om.contracts = []entities.Contract{}
		return
	} else{
		om.logger.Info("successfully loaded contracts from disk")
		contracts := make([]entities.Contract, len(tmp_contracts))
		for i:=0; i<len(tmp_contracts); i++{
			contracts[i].OwnerID = tmp_contracts[i].OwnerID
			contracts[i].Size = uint32(tmp_contracts[i].Size)
			contracts[i].Cid = tmp_contracts[i].Cid
			contracts[i].Expires = tmp_contracts[i].Expires
			contracts[i].Cardinality = uint32(tmp_contracts[i].Cardinality)
			contracts[i].Datatype = uint32(tmp_contracts[i].Datatype)
		}
		om.contracts = contracts
		return
	}

	om.contracts = []entities.Contract{}
}


//periodically check for expired contracts and remove them
func (om *OfferManager) contractTrimmer(){
	tick := time.Tick(23 * time.Second)

	for{
		<-tick

		var toTrim []int
		for ix, c := range om.contracts{
			if c.Expires.Before(time.Now()){
				toTrim = append(toTrim, ix)
			}
		}

		if len(toTrim) > 0{
			func(){
				om.lock.Lock()
				defer om.lock.Unlock()

				om.logger.Info("trimming expired contracts")
				//remove expired contracts and make new contracts array
				newContracts := make([]entities.Contract, len(om.contracts)-len(toTrim))
				toTrim = append(toTrim, -1)
				newContractsIx, toTrimIx := 0, 0
				for ix, c := range om.contracts{
					if ix == toTrim[toTrimIx]{
						om.logger.Info("trimed expired contract", zap.String("OwnerID", om.contracts[ix].OwnerID))
						go ipfs.IpfsUnpinAll(om.contracts[ix].Cid, om.logger)
						if om.isOfferSet{
							om.offer.Capacity += om.contracts[ix].Size
							om.logger.Info("Offering capacity increased to: ", zap.Int("cap", int(om.offer.Capacity)))
						}
						toTrimIx++
					} else if toTrim[toTrimIx] == -1{
						break
					} else{
						newContracts[newContractsIx] = c
						newContractsIx++
					}
				}
				om.contracts = newContracts
				om.writeToDisk(true)
			}()
		}
	}



}

func (om *OfferManager) writeToDisk(locked bool) error{
	if !locked{
		om.lock.Lock()
		defer om.lock.Unlock()
	}

	json, err := json.MarshalIndent(&om.contracts, "", " ")
	if err != nil{
		return errors.Wrap(err, "marshalling contracts")
	}
	filepath := contracts_filepath + om.node.ID().Pretty() + "_contracts.json"
	ioutil.WriteFile(filepath, json, 0644)
	om.logger.Info("contracts written to disk", zap.String("filepath", filepath))
	return nil
}

func (om *OfferManager) storeContract(contract entities.Contract, locked bool) error{
	if !locked{
		om.lock.Lock()
		defer om.lock.Unlock()
	}

	om.contracts = append(om.contracts, contract)
	if err := om.writeToDisk(true); err != nil{
		om.logger.Warn("failed writing contracts to disk")
		return err
	}

	return nil
}

func (om *OfferManager) ProcessStorageRequest(request messages.StorageRequest) messages.StorageResponse{
	resp := messages.StorageResponse{
		Multiaddr: om.node.Multiaddr(),
	}

	if om.isOfferSet && om.offer.Capacity > request.Contract.Size{
		om.logger.Info("pining content from storage request")
		if err := ipfs.HostContent(request.Contract.Cid, om.logger); err != nil{
			om.logger.Warn("failed to host content", zap.Error(err))
			resp.Done = false
			return resp
		}
		om.logger.Info("pining content from storage request: DONE")

		om.lock.Lock()
		defer om.lock.Unlock()

		om.logger.Info("storing contract from storage request")
		if err := om.storeContract(request.Contract, true); err != nil{
			om.logger.Warn("storing contract: FAILED", zap.Error(err))
			resp.Done = false
			return resp
		}
		om.logger.Info("storing contract from storage request: DONE")

		om.offer.Capacity -= request.Contract.Size
		om.logger.Info("Offering capacity decreased to: ", zap.Int("cap", int(om.offer.Capacity)))

		resp.Done = true
		return resp
	} else{
		resp.Done = false
		return resp
	}
}

func (om *OfferManager) ProcessRetrievalRequest(request messages.RetrievalRequest) messages.RetrievalResponse{
	om.logger.Info("processing data request")
	defer om.logger.Info("processing data request: DONE")

	resp := messages.RetrievalResponse{}
	//CHECK TIMESTAMP N SHEIT
	expires := request.Timestamp.Add(6 * time.Minute)
	if expires.Before(time.Now()){
		om.logger.Info("data retrieval request expired; iscarding")
		return resp
	}

	om.lock.Lock()
	defer om.lock.Unlock()

	var ipfsLinks []string
	for _, contract := range om.contracts{
		if contract.OwnerID == request.OwnerID{
			ipfsLinks = append(ipfsLinks, contract.Cid)
		}
	}

	if len(ipfsLinks) > 0{
		om.logger.Info("found matching contract(s)")
	}

	resp.IpfsLinks = ipfsLinks
	return resp
}


func (om *OfferManager) SetOffer(capacity uint32) error{
	if om.isOfferSet{
		om.offer.Capacity = capacity
		om.logger.Info("offer capacity sucessfully changed")
		return errors.New("offer is already set")
	}
	om.lock.Lock()
	defer om.lock.Unlock()

	om.offer.Multiaddr = om.node.Multiaddr()
	om.offer.Capacity = capacity
	om.offer.Timestamp = time.Now()
	om.offer.Signature = "signed: lmao"
	om.isOfferSet = true
	om.logger.Info("offer successfully set")

	return nil
}

func (om *OfferManager) IsOfferSet() bool{
	return om.isOfferSet
}

func (om *OfferManager) updateOffer(locked bool){
	if !locked{
		om.lock.Lock()
		defer om.lock.Unlock()
	}

	om.offer.Multiaddr = om.node.Multiaddr()
	om.offer.Timestamp = time.Now()
	str := om.offer.Multiaddr + strconv.FormatUint(uint64(om.offer.Capacity), 10) + strconv.FormatInt(om.offer.Timestamp.Unix(), 10)
	signature, err := om.node.sign(om.node.ID().Pretty(), []byte(str))
	if err != nil{
		om.logger.Error("couldn't sign offer", zap.Error(err))
		return
	}


	om.offer.Signature = string(signature)
	/*
	fmt.Printf("\n\nthis is offer str:  %v\n\n", str)
	fmt.Printf("\n\nthis is offer signature:  %v\n\n", om.offer.Signature)
	fmt.Printf("\n\nevoga kurba this is: %v\n\n", om.node.verify(om.offer))
	*/
}

func (om *OfferManager) GetContracts() []entities.Contract{
	if len(om.contracts) == 0{
		return nil
	}
	return om.contracts
}

func (om *OfferManager) GetOffer(locked bool) entities.Offer{
	om.updateOffer(locked)
	return om.offer
}

func (om *OfferManager) Lock(){
	om.lock.Lock()
}

func (om *OfferManager) Unlock(){
	om.lock.Unlock()
}
