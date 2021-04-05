package events

import(
	_"time"

	genapi "omnidisk/gen/api"
	"omnidisk/entities"
)

//Event represents a node event.
type Event interface{
	//MarshalToProtobuf maps events into API protobuf events
	MarshalToProtobuf() *genapi.Event
}


//OfferPub occurs when a new publication of an offer is received
type OfferPub struct{
	Offer entities.Offer
}

func (e *OfferPub) MarshalToProtobuf() *genapi.Event{
	return &genapi.Event{
		Type: genapi.Event_OFFER_PUB,
		OfferPub: &genapi.EvtOfferPub{
			Offer: &genapi.Offer{
				Multiaddr:	e.Offer.Multiaddr,
				Capacity:	e.Offer.Capacity,
			},
		},
	}
}


//ContractSub occurs when a new publication of a contract is received
type ContractPub struct{
	Multiaddr string
	Contracts []entities.Contract
}

func (e *ContractPub) MarshalToProtobuf() *genapi.Event{

	contracts := make([]genapi.Contract, len(e.Contracts))

	for i:=0; i<len(e.Contracts); i++{
		contracts[i].OwnerId =		e.Contracts[i].OwnerID
		contracts[i].Size =			e.Contracts[i].Size
		contracts[i].Cid =			e.Contracts[i].Cid
		contracts[i].Expires =		e.Contracts[i].Expires.Unix()
		contracts[i].Cardinality =	e.Contracts[i].Cardinality
		contracts[i].Datatype =		e.Contracts[i].Datatype
	}

	pcontracts := make([]*genapi.Contract, len(e.Contracts))
	for i:=0; i<len(e.Contracts); i++{
		pcontracts[i] = &(contracts[i])
	}

	return &genapi.Event{
		Type: genapi.Event_CONTRACT_PUB,
		ContractPub: &genapi.EvtContractPub{
			Multiaddr:	e.Multiaddr,
			Contracts:	pcontracts,
		},
	}
}


//occurs on an offering node when it's contacted by a storage-seeking node
type StorageRequest struct{
	Multiaddr string
	Contract entities.Contract
}

func (e *StorageRequest) MarshalToProtobuf() *genapi.Event{
	return &genapi.Event{
		Type: genapi.Event_STORAGE_REQUEST,
		StorageRequest: &genapi.EvtStorageRequest{
			Multiaddr:	e.Multiaddr,
			Contract:	&genapi.Contract{
				OwnerId:			e.Contract.OwnerID,
				Size:				e.Contract.Size,
				Cid:				e.Contract.Cid,
				Expires:			e.Contract.Expires.Unix(),
				Cardinality:	e.Contract.Cardinality,
				Datatype:		e.Contract.Datatype,
			},
		},
	}
}


//occurs on storage-seeking node on getting a response to its request
type StorageResponse struct{
	Multiaddr string
	Done bool
}

func (e *StorageResponse) MarshalToProtobuf() *genapi.Event{
	return &genapi.Event{
		Type: genapi.Event_STORAGE_RESPONSE,
		StorageResponse: &genapi.EvtStorageResponse{
			Multiaddr:	e.Multiaddr,
			Done:			e.Done,
		},
	}
}


//occurs on storing node when owner requests it's data back
type RetrievalRequest struct{
	Multiaddr string
}

func (e *RetrievalRequest) MarshalToProtobuf() *genapi.Event{
	return &genapi.Event{
		Type: genapi.Event_RETRIEVAL_REQUEST,
		RetrievalRequest: &genapi.EvtRetrievalRequest{
			Multiaddr:	e.Multiaddr,
		},
	}
}


//occurs on node after receiving reply to its request for data retrieval
type RetrievalResponse struct{
	IpfsLinks []string
}

func (e *RetrievalResponse) MarshalToProtobuf() *genapi.Event{
	return &genapi.Event{
		Type: genapi.Event_RETRIEVAL_RESPONSE,
		RetrievalResponse: &genapi.EvtRetrievalResponse{
			IpfsLinks:	e.IpfsLinks,
		},
	}
}
