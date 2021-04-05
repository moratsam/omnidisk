package node

import(
	"context"
	_"fmt"
	"sync"
	_"time"

	ggio "github.com/gogo/protobuf/io"
	libp2phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"

	"omnidisk/events"
	genmsg "omnidisk/gen/msg"
	"omnidisk/messages"
)



const(
	protocolID = "/lmaao"
)

type Connector struct{
	logger *zap.Logger
	node Node
	host libp2phost.Host

	offerManager *OfferManager

	eventPublisher events.Publisher

	lock sync.RWMutex
}


func NewConnector(logger *zap.Logger, node Node, host libp2phost.Host, om *OfferManager) (*Connector, events.Subscriber){
	if logger == nil{
		logger = zap.NewNop()
	}

	evtPub, evtSub := events.NewSubscription()

	c := &Connector{
		logger:				logger,
		node:					node,
		host:					host,
		offerManager:		om,
		eventPublisher:	evtPub,
	}

	c.host.SetStreamHandler(protocolID, func(s network.Stream){
		go c.reader(s)
	})


	return c, evtSub
}

func (c *Connector) publishEvent(msg interface{}){
	switch m := msg.(type){
		case messages.StorageRequest:
			evt := &events.StorageRequest{
						Multiaddr:	m.Multiaddr,
						Contract:	m.Contract,
					 }
			if err := c.eventPublisher.Publish(evt); err != nil{
				c.logger.Warn("couldn't publish storage request event")
			}

		case messages.StorageResponse:
			evt := &events.StorageResponse{
				Multiaddr:	m.Multiaddr,
				Done:			m.Done,
			}
			if err := c.eventPublisher.Publish(evt); err != nil{
				c.logger.Warn("couldn't publish storage response event")
			}

		case messages.RetrievalRequest:
			evt := &events.RetrievalRequest{
				Multiaddr:	m.Multiaddr,
			}
			if err := c.eventPublisher.Publish(evt); err != nil{
				c.logger.Warn("couldn't publish retrieval request event")
			}

		case messages.RetrievalResponse:
			evt := &events.RetrievalResponse{
				IpfsLinks: m.IpfsLinks,
			}
			if err := c.eventPublisher.Publish(evt); err != nil{
				c.logger.Warn("couldn't publish retrieval response event")
			}

		default:
	}
}

func receive(s network.Stream) (genmsg.Message, error){
	in := genmsg.Message{}
	reader := ggio.NewFullReader(s, 1000)
	if err := reader.ReadMsg(&in); err != nil{
		return in, err
	}
	return in, nil
}

//write to stream
func write(s network.Stream, out *genmsg.Message) error{
	writer := ggio.NewFullWriter(s)
	if err := writer.WriteMsg(out); err != nil{
		return err
	}

	return nil
}


func (c *Connector) reader(s network.Stream){
	defer s.Close()

	//receive from stream
	in, err := receive(s)
	if err != nil{
		c.logger.Warn("couldn't read request from stream", zap.Error(err))
		return
	}

	var out *genmsg.Message
	switch in.Type{
		case genmsg.Message_STORAGE_REQUEST:
			c.logger.Info("received storage request")
			//publish node event
			msg := messages.UnmarshalFromProtobuf(&in).(messages.StorageRequest)
			c.publishEvent(msg)

			//process storage request
			resp := c.offerManager.ProcessStorageRequest(msg)
			out = resp.MarshalToProtobuf()

		case genmsg.Message_RETRIEVAL_REQUEST:
			c.logger.Info("received retrieval request")
			//publish node event
			msg := messages.UnmarshalFromProtobuf(&in).(messages.RetrievalRequest)
			if ver := c.node.verify(msg); ver != true{
				c.logger.Debug("received non-verifiable retrieval request")
			}
			c.publishEvent(msg)

			//process data retrieval request
			resp := c.offerManager.ProcessRetrievalRequest(msg)
			out = resp.MarshalToProtobuf()
		default:
			c.logger.Debug("received request with unknown type")
			return
	}

	//write to stream
	if err := write(s, out); err != nil{
		c.logger.Warn("couldn't write response to stream", zap.Error(err))
		return
	}
	c.logger.Debug("", zap.String("successfully sent response to: ", s.ID()))
}


func (c *Connector) Send(addr string, request interface{}, ack *int, response interface{}){
	var out *genmsg.Message


	switch r := request.(type){
		case messages.StorageRequest:
			out = r.MarshalToProtobuf()
		case messages.RetrievalRequest:
			out = r.MarshalToProtobuf()
		default:
			c.logger.Warn("request type unknown")
			*ack = 0
			return
	}

	//parse multiaddr
	ma, err := multiaddr.NewMultiaddr(addr)
	if err != nil{
		c.logger.Warn("couldnt parse multiaddr", zap.Error(err))
		*ack = 0
		return
	}
	addrInfo, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil{
		c.logger.Warn("couldnt get info from multiaddr", zap.Error(err))
		return
		*ack = 0
	}

	//connect
	if err := c.host.Connect(context.Background(), *addrInfo); err != nil{
		c.logger.Warn("couldnt establish connection", zap.Error(err))
		return
		*ack = 0
	}
	c.logger.Debug("", zap.String("contact established to: ", addrInfo.String()))

	//open stream
	s, err := c.host.NewStream(context.Background(), addrInfo.ID, protocolID)
	if err != nil{
		c.logger.Warn("couldn't open stream", zap.Error(err))
		*ack = 0
		return
	}
	defer s.Close()

	//write
	if err := write(s, out); err != nil{
		c.logger.Warn("couldn't write request to stream", zap.Error(err))
		*ack = 0
		return
	}
	c.logger.Debug("", zap.String("successfully sent request to: ", addrInfo.String()))

	c.logger.Debug("awaiting response..")
	//receive from stream
	in, err := receive(s)
	c.logger.Debug("response received")
	if err != nil{
		c.logger.Warn("couldn't read response from stream", zap.Error(err))
		*ack = 0
		return
	}

	//unpack values
	if out.Type == genmsg.Message_STORAGE_REQUEST && in.Type == genmsg.Message_STORAGE_RESPONSE{
			c.logger.Debug("response is type STORAGE RESPONSE")
			msg := messages.UnmarshalFromProtobuf(&in).(messages.StorageResponse)
			response.(*messages.StorageResponse).Multiaddr = msg.Multiaddr
			response.(*messages.StorageResponse).Done = msg.Done

			c.publishEvent(msg)
		if msg.Done == true{
			*ack = 1
		} else{
			*ack = 0
		}
	} else if out.Type == genmsg.Message_RETRIEVAL_REQUEST && in.Type == genmsg.Message_RETRIEVAL_RESPONSE{
			c.logger.Debug("response is type RETRIEVAL RESPONSE")
			msg := messages.UnmarshalFromProtobuf(&in).(messages.RetrievalResponse)
			response.(*messages.RetrievalResponse).IpfsLinks = msg.IpfsLinks

			c.publishEvent(msg)
			if len(msg.IpfsLinks) > 0{
				*ack = 1
			} else{
				*ack = 0
			}
	} else{
		c.logger.Warn("unexpected response type")
		*ack = 0
		return
	}
}
