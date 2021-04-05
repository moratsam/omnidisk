package messages

import(
	"time"

	genmsg "omnidisk/gen/msg"
	"omnidisk/entities"
)

//Message represents a node event.
type Message interface{
	//MarshalToProtobuf maps events into API protobuf events
	MarshalToProtobuf() *genmsg.Message
}

//occurs on an offering node when it's contacted by a storage-seeking node
type StorageRequest struct{
	Multiaddr string
	Contract entities.Contract
}

func (m *StorageRequest) MarshalToProtobuf() *genmsg.Message{
	return &genmsg.Message{
		Type: genmsg.Message_STORAGE_REQUEST,
		StorageRequest: &genmsg.MsgStorageRequest{
			Multiaddr:	m.Multiaddr,
			Contract:	&genmsg.Contract{
				OwnerId:			m.Contract.OwnerID,
				Size_:			m.Contract.Size,
				Cid:				m.Contract.Cid,
				Expires:			m.Contract.Expires.Unix(),
				Cardinality:	m.Contract.Cardinality,
				Datatype:		m.Contract.Datatype,
			},
		},
	}
}

//occurs on storage-seeking node on getting a response to its request
type StorageResponse struct{
	Multiaddr string
	Done bool
}

func (m *StorageResponse) MarshalToProtobuf() *genmsg.Message{
	return &genmsg.Message{
		Type: genmsg.Message_STORAGE_RESPONSE,
		StorageResponse: &genmsg.MsgStorageResponse{
			Multiaddr:	m.Multiaddr,
			Done:			m.Done,
		},
	}
}


//occurs on storing node when owner requests it's data back
type RetrievalRequest struct{
	Multiaddr	string
	OwnerID		string
	Timestamp	time.Time
	Signature	string
}

func (m *RetrievalRequest) MarshalToProtobuf() *genmsg.Message{
	return &genmsg.Message{
		Type: genmsg.Message_RETRIEVAL_REQUEST,
		RetrievalRequest: &genmsg.MsgRetrievalRequest{
			Multiaddr:	m.Multiaddr,
			OwnerId:		m.OwnerID,
			Timestamp:	m.Timestamp.Unix(),
			Signature:	m.Signature,
		},
	}
}


//occurs on node after receiving reply to its request for data retrieval
type RetrievalResponse struct{
	IpfsLinks []string
}

func (m *RetrievalResponse) MarshalToProtobuf() *genmsg.Message{
	return &genmsg.Message{
		Type: genmsg.Message_RETRIEVAL_RESPONSE,
		RetrievalResponse: &genmsg.MsgRetrievalResponse{
			IpfsLinks:	m.IpfsLinks,
		},
	}
}


func UnmarshalFromProtobuf(msg *genmsg.Message) interface{} {
	switch msg.Type{
		case genmsg.Message_STORAGE_REQUEST:
			contract := entities.Contract{
				OwnerID:			msg.StorageRequest.Contract.OwnerId,
				Size:				msg.StorageRequest.Contract.Size_,
				Cid:				msg.StorageRequest.Contract.Cid,
				Expires:			time.Unix(msg.StorageRequest.Contract.Expires, 0),
				Cardinality:	msg.StorageRequest.Contract.Cardinality,
				Datatype:		msg.StorageRequest.Contract.Datatype,
			}

			return StorageRequest{
				Multiaddr:		msg.StorageRequest.Multiaddr,
				Contract:		contract,
			}

		case genmsg.Message_STORAGE_RESPONSE:
			return StorageResponse{
				Multiaddr:	msg.StorageResponse.Multiaddr,
				Done:			msg.StorageResponse.Done,
			}

		case genmsg.Message_RETRIEVAL_REQUEST:
			return RetrievalRequest{
				Multiaddr:	msg.RetrievalRequest.Multiaddr,
				OwnerID:		msg.RetrievalRequest.OwnerId,
				Timestamp:	time.Unix(msg.RetrievalRequest.Timestamp, 0),
				Signature:	msg.RetrievalRequest.Signature,
			}

		case genmsg.Message_RETRIEVAL_RESPONSE:
			return RetrievalResponse{
				IpfsLinks:	msg.RetrievalResponse.IpfsLinks,
			}
	}

	return nil
}


