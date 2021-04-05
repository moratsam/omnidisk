package entities

import(
	"time"
)

type Offer struct{
	Multiaddr	string		`json:"multiaddr"`
	Capacity		uint32		`json:"capacity"`
	Timestamp	time.Time	`json:"timestamp"`
	Signature	string		`json:"signature"`
}

type Contract struct{
	OwnerID		string		`json:"owner_id"`
	Size			uint32		`json:"size"`
	Cid			string		`json:"cid"`
	Expires		time.Time	`json:"expires"`
	Cardinality	uint32		`json:"cardinality"`
	Datatype		uint32		`json:"datatype"`
}

